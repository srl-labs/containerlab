// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/awalterschulze/gographviz"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	e "github.com/srl-labs/containerlab/errors"
	"github.com/srl-labs/containerlab/internal/mermaid"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	dockerC "github.com/docker/docker/client"
	"golang.org/x/term"
)

type GraphTopo struct {
	Nodes []types.ContainerDetails `json:"nodes,omitempty"`
	Links []Link                   `json:"links,omitempty"`
}

type Link struct {
	Source         string `json:"source,omitempty"`
	SourceEndpoint string `json:"source_endpoint,omitempty"`
	Target         string `json:"target,omitempty"`
	TargetEndpoint string `json:"target_endpoint,omitempty"`
}

type TopoData struct {
	Name string
	Data template.JS
}

// noListFs embeds the http.Dir to override the Open method of a filesystem
// to prevent listing of static files, see https://github.com/srl-labs/containerlab/pull/802#discussion_r815373751
type noListFs struct {
	http.FileSystem
}

var g *gographviz.Graph

// GenerateDotGraph generates a graph of the lab topology.
func (c *CLab) GenerateDotGraph() error {
	log.Info("Generating lab graph...")
	g = gographviz.NewGraph()
	if err := g.SetName(c.TopoPaths.TopologyFilenameWithoutExt()); err != nil {
		return err
	}
	if err := g.SetDir(false); err != nil {
		return err
	}

	var attr map[string]string

	// Process the Nodes
	for nodeName, node := range c.Nodes {
		attr = make(map[string]string)
		attr["color"] = "red"
		attr["style"] = "filled"
		attr["fillcolor"] = "red"

		attr["label"] = nodeName
		attr["xlabel"] = node.Config().Kind
		if strings.TrimSpace(node.Config().Group) != "" {
			attr["group"] = node.Config().Group
			if strings.Contains(node.Config().Group, "bb") {
				attr["fillcolor"] = "blue"
				attr["color"] = "blue"
				attr["fontcolor"] = "white"
			} else if strings.Contains(node.Config().Kind, "srl") {
				attr["fillcolor"] = "green"
				attr["color"] = "green"
				attr["fontcolor"] = "black"
			}
		}
		if err := g.AddNode(c.TopoPaths.TopologyFilenameWithoutExt(),
			node.Config().ShortName, attr); err != nil {
			return err
		}

	}

	// Process the links inbetween Nodes
	for _, link := range c.Links {
		attr = make(map[string]string)
		attr["color"] = "black"

		eps := link.GetEndpoints()
		ANodeName := eps[0].GetNode().GetShortName()
		BNodeName := eps[1].GetNode().GetShortName()

		if (strings.Contains(ANodeName, "client")) ||
			(strings.Contains(BNodeName, "client")) {
			attr["color"] = "blue"
		}
		if err := g.AddEdge(ANodeName, BNodeName, false, attr); err != nil {
			return err
		}
		// log.Info(link.A.Node.ShortName, " <-> ", link.B.Node.ShortName)
	}

	// create graph directory
	utils.CreateDirectory(c.TopoPaths.TopologyLabDir(), 0755)
	utils.CreateDirectory(c.TopoPaths.GraphDir(), 0755)

	// create graph filename
	dotfile := c.TopoPaths.GraphFilename(".dot")
	utils.CreateFile(dotfile, g.String())
	log.Infof("Created %s", dotfile)

	pngfile := c.TopoPaths.GraphFilename(".png")

	// Only try to create png
	if commandExists("dot") {
		err := generatePngFromDot(dotfile, pngfile)
		if err != nil {
			return err
		}
		log.Info("Created ", pngfile)
	}
	return nil
}

// generatePngFromDot generated PNG from the provided dot file.
func generatePngFromDot(dotfile string, outfile string) (err error) {
	_, err = exec.Command("dot", "-o", outfile, "-Tpng", dotfile).CombinedOutput()
	if err != nil {
		log.Errorf("failed to generate png (%v) from dot file (%v), with error (%v)", outfile, dotfile, err)
		return fmt.Errorf("failed to generate png (%v) from dot file (%v), with error (%v)", outfile, dotfile, err)
	}
	return nil
}

// commandExists checks for the existence of the given command on the system.
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	if err == nil {
		log.Debugf("executable %s exists!", cmd)
	} else {
		log.Debugf("executable %s doesn't exist!", cmd)
	}
	return err == nil
}

// Open is a custom FS opener that prevents listing of the files in the filesystem
// see https://github.com/srl-labs/containerlab/pull/802#discussion_r815373751
func (nfs noListFs) Open(name string) (result http.File, err error) {
	f, err := nfs.FileSystem.Open(name)
	if err != nil {
		return
	}

	stat, err := f.Stat()
	if err != nil {
		return
	}

	if stat.IsDir() {
		return nil, os.ErrNotExist
	}

	return f, nil
}

func buildGraphNode(node nodes.Node) types.ContainerDetails {
	return types.ContainerDetails{
		Name:        node.Config().ShortName,
		Kind:        node.Config().Kind,
		Image:       node.Config().Image,
		Group:       node.Config().Group,
		State:       "N/A",
		IPv4Address: node.Config().MgmtIPv4Address,
		IPv6Address: node.Config().MgmtIPv6Address,
	}
}

func (c *CLab) BuildGraphFromTopo(g *GraphTopo) {
	log.Info("building graph from topology file")
	for _, node := range c.Nodes {
		g.Nodes = append(g.Nodes, buildGraphNode(node))
	}
}

func (c *CLab) BuildGraphFromDeployedLab(g *GraphTopo, containers []runtime.GenericContainer) {
	containerNames := make(map[string]struct{})
	for _, cont := range containers {
		log.Debugf("looking for node name %s", cont.Labels[labels.NodeName])
		if node, ok := c.Nodes[cont.Labels[labels.NodeName]]; ok {
			containerNames[node.Config().ShortName] = struct{}{}
			g.Nodes = append(g.Nodes, types.ContainerDetails{
				Name:        node.Config().ShortName,
				Kind:        node.Config().Kind,
				Image:       node.Config().Image,
				Group:       node.Config().Group,
				State:       fmt.Sprintf("%s/%s", cont.State, cont.Status),
				IPv4Address: cont.GetContainerIPv4(),
				IPv6Address: cont.GetContainerIPv6(),
			})
		}
	}
	for _, node := range c.Nodes {
		if _, exist := containerNames[node.Config().ShortName]; !exist {
			g.Nodes = append(g.Nodes, buildGraphNode(node))
		}
	}
}

func (c *CLab) GenerateMermaidGraph(direction string) error {
	fc := mermaid.NewFlowChart()

	fc.SetTitle(c.Config.Name)

	if err := fc.SetDirection(direction); err != nil {
		return err
	}

	// Process the links between Nodes
	for _, link := range c.Links {
		eps := link.GetEndpoints()
		fc.AddEdge(eps[0].GetNode().GetShortName(), eps[1].GetNode().GetShortName())
	}

	// create graph directory
	utils.CreateDirectory(c.TopoPaths.TopologyLabDir(), 0755)
	utils.CreateDirectory(c.TopoPaths.GraphDir(), 0755)

	// create graph filename
	fname := c.TopoPaths.GraphFilename(".mermaid")

	// Generate graph
	var w strings.Builder
	fc.Generate(&w)
	utils.CreateFile(fname, w.String())

	log.Infof("Created mermaid diagram file: %s", fname)

	return nil
}

//go:embed graph_templates/nextui/nextui.html
var defaultTemplate string

//go:embed graph_templates/nextui/static
var defaultStatic embed.FS

func (c *CLab) ServeTopoGraph(tmpl, staticDir, srv string, topoD TopoData) error {
	var t *template.Template

	if tmpl == "" {
		t = template.Must(template.New("nextui.html").Parse(defaultTemplate))
	} else if utils.FileExists(tmpl) {
		t = template.Must(template.ParseFiles(tmpl))
	} else {
		return fmt.Errorf("%w. Path %s", e.ErrFileNotFound, tmpl)
	}

	if staticDir != "" && tmpl == "" {
		return fmt.Errorf("the --static-dir flag must be used with the --template flag")
	}

	var staticFS http.FileSystem
	if staticDir == "" {
		// extract the sub fs with static files from the embedded fs
		subFS, err := fs.Sub(defaultStatic, "graph_templates/nextui/static")
		if err != nil {
			return err
		}

		staticFS = http.FS(subFS)
	} else {
		log.Infof("Serving static files from directory: %s", staticDir)
		staticFS = http.Dir(staticDir)
	}

	fs := http.FileServer(noListFs{staticFS})
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = t.Execute(w, topoD)
	})

	log.Infof("Serving topology graph on http://%s", srv)

	return http.ListenAndServe(srv, nil)
}

// GenerateDrawioDiagram pulls (if needed) and runs the "clab-io-draw" container in interactive TTY mode.
// The container is removed automatically when the TUI session ends.
func (c *CLab) GenerateDrawioDiagram(version string, userArgs []string) error {
	cli, err := dockerC.NewClientWithOpts(dockerC.FromEnv, dockerC.WithAPIVersionNegotiation())
	if err != nil {
		log.Errorf("Failed to create Docker client: %v", err)
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	ctx := context.Background()
	imageName := fmt.Sprintf("ghcr.io/srl-labs/clab-io-draw:%s", version)

	// If user asks for "latest" => always pull. Otherwise only if missing.
	if version == "latest" {
		log.Infof("Forcing a pull of the latest image: %s", imageName)
		if err := forcePull(ctx, cli, imageName); err != nil {
			return fmt.Errorf("failed to pull latest image: %w", err)
		}
	} else {
		if err := pullImageIfNotPresent(ctx, cli, imageName); err != nil {
			return fmt.Errorf("could not ensure image presence: %w", err)
		}
	}

	topoFile := c.TopoPaths.TopologyFilenameBase()

	// Turn user-supplied arguments into properly tokenized slice
	parsedArgs := parseDrawioArgs(userArgs)
	cmdArgs := append([]string{"-i", topoFile}, parsedArgs...)

	log.Infof("Launching clab-io-draw version=%s with arguments: %v", version, cmdArgs)

	// Create the container in TTY mode with an open STDIN
	createResp, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:     imageName,
			Cmd:       cmdArgs,
			Tty:       true,
			OpenStdin: true,
			Env:       []string{"TERM=xterm-256color"},
		},
		&container.HostConfig{
			Binds: []string{
				fmt.Sprintf("%s:/data", c.TopoPaths.TopologyFileDir()),
			},
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		log.Errorf("Failed to create container for clab-io-draw: %v", err)
		return fmt.Errorf("failed to create container: %w", err)
	}
	containerID := createResp.ID

	// Attach to TTY
	attachResp, err := cli.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		log.Errorf("Failed to attach to container: %v", err)
		return fmt.Errorf("failed to attach to container: %w", err)
	}
	defer attachResp.Close()

	// Start the container
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		log.Errorf("Failed to start container: %v", err)
		return fmt.Errorf("failed to start container: %w", err)
	}

	// If we're running in a real terminal, set raw mode & handle resizing
	inTerminal := term.IsTerminal(int(os.Stdin.Fd()))
	var oldState *term.State
	if inTerminal {
		oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			log.Warnf("Unable to set terminal to raw mode: %v", err)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for s := range sigCh {
			switch s {
			case syscall.SIGWINCH:
				if inTerminal {
					resizeDockerTTY(cli, ctx, containerID)
				}
			case syscall.SIGINT, syscall.SIGTERM:
				log.Infof("Received signal %v, stopping container %s", s, containerID)
				timeoutSec := 2
				_ = cli.ContainerStop(ctx, containerID,
					container.StopOptions{Timeout: &[]int{timeoutSec}[0]})
			}
		}
	}()

	if inTerminal {
		resizeDockerTTY(cli, ctx, containerID)
	}

	// Pipe local -> container
	go func() { _, _ = io.Copy(attachResp.Conn, os.Stdin) }()

	// Pipe container -> local
	errChan := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(os.Stdout, attachResp.Reader)
		errChan <- copyErr
	}()

	// Wait for container to exit
	waitCh, waitErrCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var exitCode int64
	select {
	case we := <-waitErrCh:
		if we != nil {
			log.Errorf("Error waiting for container: %v", we)
			return fmt.Errorf("error waiting for container: %w", we)
		}
	case status := <-waitCh:
		if status.Error != nil {
			log.Errorf("Container wait error: %s", status.Error.Message)
			return fmt.Errorf("container wait error: %s", status.Error.Message)
		}
		exitCode = status.StatusCode
	}

	// If copying container output ended in an error, log it
	if cerr := <-errChan; cerr != nil && cerr != io.EOF {
		log.Warnf("Error reading container output: %v", cerr)
	}

	// Restore terminal state if needed
	if oldState != nil {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
	}

	// Remove container
	removeOpts := container.RemoveOptions{Force: true}
	if err := cli.ContainerRemove(ctx, containerID, removeOpts); err != nil {
		log.Warnf("Failed to remove container %s: %v", containerID, err)
	}

	if exitCode != 0 {
		return fmt.Errorf("clab-io-draw container exited with code %d", exitCode)
	}
	log.Info("Diagram created successfully.")
	return nil
}

// forcePull always does a Docker Pull, even if the image is already present locally.
func forcePull(ctx context.Context, cli *dockerC.Client, imageName string) error {
	log.Infof("Pulling image %q forcibly", imageName)
	rc, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %q: %w", imageName, err)
	}
	defer rc.Close()
	// Must consume entire body or Docker won't finalize the pull
	_, _ = io.Copy(io.Discard, rc)
	return nil
}

// pullImageIfNotPresent does an Inspect first. If not found, does a pull.
// If found, just logs that it's skipping.
func pullImageIfNotPresent(ctx context.Context, cli *dockerC.Client, imageName string) error {
	_, _, err := cli.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		// Found locally
		log.Debugf("Image %q already present locally; skipping pull", imageName)
		return nil
	}
	if dockerC.IsErrNotFound(err) {
		log.Infof("Image %q not found locally; pulling...", imageName)
		rc, perr := cli.ImagePull(ctx, imageName, image.PullOptions{})
		if perr != nil {
			return fmt.Errorf("failed to pull image %q: %w", imageName, perr)
		}
		defer rc.Close()
		_, _ = io.Copy(io.Discard, rc)
		return nil
	}
	return fmt.Errorf("failed to inspect image %q: %w", imageName, err)
}

// resizeDockerTTY attempts to match the container's TTY size to the local terminal size.
// Called on startup and whenever SIGWINCH is received.
func resizeDockerTTY(cli *dockerC.Client, ctx context.Context, containerID string) {
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		log.Debugf("Unable to get local terminal size: %v", err)
		return
	}
	if resizeErr := cli.ContainerResize(ctx, containerID, container.ResizeOptions{
		Width:  uint(w),
		Height: uint(h),
	}); resizeErr != nil {
		log.Debugf("Failed to resize container TTY: %v", resizeErr)
	}
}

func parseDrawioArgs(argList []string) []string {
	// If the user passes multiple tokens in one argument, e.g. "-I --theme nokia_modern",
	// we'll parse them into separate tokens.
	var finalTokens []string
	for _, rawArg := range argList {
		parsed, err := shlex.Split(rawArg)
		if err != nil {
			// If splitting fails, fallback to using the entire rawArg
			log.Warnf("Failed to parse %q via shlex; using as a single token", rawArg)
			finalTokens = append(finalTokens, rawArg)
		} else {
			finalTokens = append(finalTokens, parsed...)
		}
	}
	return finalTokens
}
