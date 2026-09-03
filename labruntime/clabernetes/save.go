package clabernetes

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// The direct device pod mounts the accepted plan and the per-node artifact roots at fixed
	// paths; the device container's PostStart command is the typed lifecycle entrypoint that,
	// re-run with the Save phase, executes the imported containerlab SaveConfig for its node.
	devicePlanPath          = "/var/lib/clabernetes/lifecycle-plan/plan.json"
	deviceArtifactRoot      = "/var/lib/clabernetes/lifecycle-artifacts"
	lifecyclePhaseArgument  = "--phase"
	lifecycleSavePhaseValue = "Save"
)

func (r *Runtime) Save(
	ctx context.Context,
	req clablabruntime.SaveRequest,
) (*clablabruntime.SaveResult, error) {
	targets, namespace, err := r.targetNodes(ctx, clablabruntime.NodeRequest{
		Name:      req.Name,
		Namespace: req.Namespace,
		Nodes:     req.Nodes,
	})
	if err != nil {
		return nil, err
	}

	result := &clablabruntime.SaveResult{}
	for _, nodeName := range targets {
		pod, err := r.devicePod(ctx, req.Name, namespace, nodeName)
		if err != nil {
			return nil, err
		}

		containerName, err := r.deviceContainerName(ctx, pod, req.Name, namespace, nodeName)
		if err != nil {
			return nil, err
		}

		saveCommand, err := deviceSaveCommand(pod, containerName)
		if err != nil {
			// Not every application container carries the lifecycle entrypoint (secondary
			// chassis cards do not); fall back to any of this node's containers that does.
			containerName, saveCommand = r.saveCapableContainer(ctx, pod, req.Name, namespace, nodeName)
			if containerName == "" {
				log.Info(
					"Skipping save for clabernetes node without a save-capable container",
					"node", nodeName,
				)

				continue
			}
		}

		marker := ""
		if req.Copy {
			marker = r.placeSaveMarker(ctx, pod, containerName)
		}

		stdout, stderr, rc, err := r.execInPod(ctx, pod, containerName, saveCommand)
		if err != nil {
			return nil, err
		}

		if len(stdout) != 0 {
			log.Info(
				"clabernetes save output",
				"node",
				nodeName,
				"stdout",
				strings.TrimSpace(string(stdout)),
			)
		}
		if len(stderr) != 0 {
			log.Info(
				"clabernetes save output",
				"node",
				nodeName,
				"stderr",
				strings.TrimSpace(string(stderr)),
			)
		}
		if rc != 0 {
			return nil, fmt.Errorf("save failed for clabernetes node %s/%s/%s: rc=%d",
				namespace, req.Name, nodeName, rc)
		}

		if req.Copy {
			files, err := r.collectSavedFiles(ctx, pod, containerName, nodeName, marker)
			if err != nil {
				return nil, err
			}
			result.Files = append(result.Files, files...)
		}
	}

	return result, nil
}

// saveCapableContainer finds a container of the logical node that carries the lifecycle
// entrypoint, preferring the node's own observed containers. Returns empty values when the node
// has no save-capable container.
func (r *Runtime) saveCapableContainer(
	ctx context.Context,
	pod *corev1.Pod,
	topologyName,
	namespace,
	nodeName string,
) (string, []string) {
	namespace = r.namespaceFor(namespace)

	node, err := r.client.Resource(nodeGVR).Namespace(namespace).
		Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", nil
	}

	containers, _, _ := unstructured.NestedSlice(node.Object, "status", "directContainers")
	for _, raw := range containers {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if name == "" {
			continue
		}
		if command, err := deviceSaveCommand(pod, name); err == nil {
			return name, command
		}
	}

	return "", nil
}

// deviceSaveCommand derives the Save-phase lifecycle command for the given device container from
// its PostStart hook: the identical typed entrypoint with only the phase changed, so client and
// plan can never disagree about the save input.
func deviceSaveCommand(pod *corev1.Pod, containerName string) ([]string, error) {
	for idx := range pod.Spec.Containers {
		container := &pod.Spec.Containers[idx]
		if container.Name != containerName {
			continue
		}
		if container.Lifecycle == nil ||
			container.Lifecycle.PostStart == nil ||
			container.Lifecycle.PostStart.Exec == nil {
			return nil, fmt.Errorf("device container %q has no lifecycle command", containerName)
		}

		command := append([]string(nil), container.Lifecycle.PostStart.Exec.Command...)
		for argIdx, argument := range command {
			if argument == lifecyclePhaseArgument && argIdx+1 < len(command) {
				command[argIdx+1] = lifecycleSavePhaseValue

				return command, nil
			}
		}

		return nil, fmt.Errorf(
			"device container %q lifecycle command has no %s argument",
			containerName,
			lifecyclePhaseArgument,
		)
	}

	return nil, fmt.Errorf("device container %q was not found in pod %s/%s",
		containerName, pod.Namespace, pod.Name)
}

// devicePlanNodes is the minimal slice of the accepted device plan needed to locate a logical
// node's artifact directory.
type devicePlanNodes struct {
	Nodes []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"nodes"`
}

// placeSaveMarker records a timestamp file so the copy step can identify exactly the files the
// save hook wrote. Only the marker's mtime matters, so when the artifact root is not writable
// for the container user (non-root device images) it falls back to /tmp. An empty return
// degrades the copy to the whole node artifact directory.
func (r *Runtime) placeSaveMarker(
	ctx context.Context,
	pod *corev1.Pod,
	containerName string,
) string {
	for _, marker := range []string{
		path.Join(deviceArtifactRoot, ".clab-save-marker"),
		"/tmp/.clab-save-marker",
	} {
		_, _, rc, err := r.execInPod(ctx, pod, containerName, []string{"touch", marker})
		if err == nil && rc == 0 {
			return marker
		}

		log.Debug("failed to place save copy marker",
			"pod", pod.Name,
			"marker", marker,
			"error", err,
			"rc", rc,
		)
	}

	log.Debug("no writable save copy marker location; copying the full node directory",
		"pod", pod.Name,
	)

	return ""
}

// missingExecutable reports whether a pod exec failed because the requested binary does not
// exist in the container image. Copying saved configs shells out to cat/find/tar in the device
// container; minimal images without those utilities can still save (the lifecycle entrypoint is
// a mounted static binary) but cannot serve the copy, which then skips the node.
func missingExecutable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "executable file not found")
}

func (r *Runtime) collectSavedFiles(
	ctx context.Context,
	pod *corev1.Pod,
	containerName,
	nodeName,
	marker string,
) ([]clablabruntime.SavedFile, error) {
	artifactDir, err := r.nodeArtifactDir(ctx, pod, containerName, nodeName)
	if err != nil {
		if missingExecutable(err) {
			log.Info(
				"Skipping save copy for clabernetes node without shell utilities in its image",
				"node", nodeName,
			)

			return nil, nil
		}

		return nil, err
	}

	_, _, rc, err := r.execInPod(ctx, pod, containerName, []string{"test", "-d", artifactDir})
	if err != nil {
		if missingExecutable(err) {
			log.Info(
				"Skipping save copy for clabernetes node without shell utilities in its image",
				"node", nodeName,
			)

			return nil, nil
		}

		return nil, err
	}
	if rc != 0 {
		log.Debug("no clabernetes saved config directory found",
			"node", nodeName,
			"path", artifactDir,
		)

		return nil, nil
	}

	tarTargets := []string{"."}
	if marker != "" {
		// Copy only what the save hook just wrote, mirroring the local runtime's behavior of
		// copying the saved configuration rather than the entire node directory.
		stdout, stderr, rc, findErr := r.execInPod(ctx, pod, containerName, []string{
			"find", artifactDir, "-type", "f", "-newer", marker,
		})
		if findErr != nil {
			if missingExecutable(findErr) {
				log.Info(
					"Skipping save copy for clabernetes node without shell utilities in its image",
					"node", nodeName,
				)

				return nil, nil
			}

			return nil, findErr
		}
		if rc != 0 {
			// The save hook ran as the same user as this exec, so every file it wrote is
			// readable here; find can only stumble over pre-existing paths owned by another
			// user (e.g. root-staged payloads under a non-root device container). Keep
			// whatever it did list instead of failing the save.
			log.Debug("save copy file discovery reported errors; using partial results",
				"node", nodeName,
				"rc", rc,
				"stderr", strings.TrimSpace(string(stderr)),
			)
		}
		defer func() {
			_, _, _, _ = r.execInPod(ctx, pod, containerName, []string{"rm", "-f", marker})
		}()

		tarTargets = tarTargets[:0]
		for _, line := range strings.Split(strings.TrimSpace(string(stdout)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			relative := strings.TrimPrefix(strings.TrimPrefix(line, artifactDir), "/")
			if relative == "" {
				continue
			}
			tarTargets = append(tarTargets, relative)
		}
		if len(tarTargets) == 0 {
			log.Debug("save produced no new config files to copy", "node", nodeName)

			return nil, nil
		}
	}

	stdout, stderr, rc, err := r.execInPod(ctx, pod, containerName,
		append([]string{"tar", "cf", "-", "-C", artifactDir}, tarTargets...))
	if err != nil {
		if missingExecutable(err) {
			log.Info(
				"Skipping save copy for clabernetes node without shell utilities in its image",
				"node", nodeName,
			)

			return nil, nil
		}

		return nil, err
	}
	if rc != 0 {
		return nil, fmt.Errorf("failed to archive saved config for node %s: rc=%d stderr=%s",
			nodeName, rc, strings.TrimSpace(string(stderr)))
	}

	files, err := savedFilesFromTar(nodeName, stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read saved config archive for node %s: %w",
			nodeName, err)
	}

	return files, nil
}

// nodeArtifactDir derives the node's opaque lab directory inside the shared artifact root from
// the accepted device plan.
func (r *Runtime) nodeArtifactDir(
	ctx context.Context,
	pod *corev1.Pod,
	containerName,
	nodeName string,
) (string, error) {
	stdout, stderr, rc, err := r.execInPod(ctx, pod, containerName,
		[]string{"cat", devicePlanPath})
	if err != nil {
		return "", err
	}
	if rc != 0 {
		return "", fmt.Errorf("failed to read device plan for node %s: rc=%d stderr=%s",
			nodeName, rc, strings.TrimSpace(string(stderr)))
	}

	var plan devicePlanNodes
	if err := json.Unmarshal(stdout, &plan); err != nil {
		return "", fmt.Errorf("failed to decode device plan for node %s: %w", nodeName, err)
	}

	for _, node := range plan.Nodes {
		if node.Name == nodeName {
			digest := sha256.Sum256([]byte(node.ID))

			return path.Join(
				deviceArtifactRoot,
				"node-"+hex.EncodeToString(digest[:])[:12],
			), nil
		}
	}

	return "", fmt.Errorf("node %s is absent from the accepted device plan", nodeName)
}

func savedFilesFromTar(nodeName string, data []byte) ([]clablabruntime.SavedFile, error) {
	reader := tar.NewReader(bytes.NewReader(data))
	var files []clablabruntime.SavedFile

	for {
		header, err := reader.Next()
		switch {
		case errors.Is(err, io.EOF):
			return files, nil
		case err != nil:
			return nil, err
		}

		name, ok := cleanTarPath(header.Name)
		if !ok || name == "." {
			continue
		}

		switch header.Typeflag {
		case tar.TypeReg:
			content, err := io.ReadAll(reader)
			if err != nil {
				return nil, err
			}

			files = append(files, clablabruntime.SavedFile{
				NodeName: nodeName,
				Name:     name,
				Data:     content,
				Mode:     header.Mode,
			})
		case tar.TypeSymlink:
			files = append(files, clablabruntime.SavedFile{
				NodeName:   nodeName,
				Name:       name,
				Mode:       header.Mode,
				LinkTarget: header.Linkname,
			})
		}
	}
}

func cleanTarPath(name string) (string, bool) {
	name = strings.TrimPrefix(name, "./")
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == "" {
		return cleaned, true
	}
	if strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") || cleaned == ".." {
		return "", false
	}

	return cleaned, true
}
