package clabernetes

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
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
		pod, err := r.launcherPod(ctx, req.Name, namespace, nodeName)
		if err != nil {
			return nil, err
		}

		copyDir := ""
		command := []string{"containerlab", "save", "-t", "/clabernetes/topo.clab.yaml"}
		if req.Copy {
			copyDir = fmt.Sprintf("/tmp/clab-save-copy-%s-%s-%d",
				req.Name, nodeName, time.Now().UnixNano())
			_, _, _, _ = r.execInPod(ctx, pod, []string{"rm", "-rf", copyDir})
			command = append(command, "--copy", copyDir)
		}

		stdout, stderr, rc, err := r.execInPod(ctx, pod, command)
		if err != nil {
			return nil, err
		}

		if len(stdout) != 0 {
			log.Info("clabernetes save output", "node", nodeName, "stdout", strings.TrimSpace(string(stdout)))
		}
		if len(stderr) != 0 {
			log.Info("clabernetes save output", "node", nodeName, "stderr", strings.TrimSpace(string(stderr)))
		}
		if rc != 0 {
			return nil, fmt.Errorf("save failed for clabernetes node %s/%s/%s: rc=%d",
				namespace, req.Name, nodeName, rc)
		}

		if req.Copy {
			files, err := r.collectSavedFiles(ctx, pod, nodeName, copyDir)
			if cleanupDir := copyDir; cleanupDir != "" {
				_, _, _, _ = r.execInPod(ctx, pod, []string{"rm", "-rf", cleanupDir})
			}
			if err != nil {
				return nil, err
			}
			result.Files = append(result.Files, files...)
		}
	}

	return result, nil
}

func (r *Runtime) collectSavedFiles(
	ctx context.Context,
	pod *corev1.Pod,
	nodeName,
	copyDir string,
) ([]clablabruntime.SavedFile, error) {
	if copyDir == "" {
		return nil, nil
	}

	nodeCopyDir := path.Join(copyDir, "clab-clabernetes-"+nodeName, nodeName)
	_, _, rc, err := r.execInPod(ctx, pod, []string{"test", "-d", nodeCopyDir})
	if err != nil {
		return nil, err
	}
	if rc != 0 {
		log.Debug("no clabernetes saved config copy directory found",
			"node", nodeName,
			"path", nodeCopyDir,
		)

		return nil, nil
	}

	stdout, stderr, rc, err := r.execInPod(ctx, pod,
		[]string{"tar", "cf", "-", "-C", nodeCopyDir, "."})
	if err != nil {
		return nil, err
	}
	if rc != 0 {
		return nil, fmt.Errorf("failed to archive saved config copy for node %s: rc=%d stderr=%s",
			nodeName, rc, strings.TrimSpace(string(stderr)))
	}

	files, err := savedFilesFromTar(nodeName, stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read saved config archive for node %s: %w",
			nodeName, err)
	}

	return files, nil
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
		case tar.TypeReg, tar.TypeRegA:
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
