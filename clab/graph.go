// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
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
	"github.com/creack/pty"
	log "github.com/sirupsen/logrus"
	e "github.com/srl-labs/containerlab/errors"
	"github.com/srl-labs/containerlab/internal/mermaid"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
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

func (c *CLab) GenerateDrawioDiagram(version string, additionalFlags []string) error {
	topoFile := c.TopoPaths.TopologyFilenameBase()

	imageName := fmt.Sprintf("ghcr.io/srl-labs/clab-io-draw:%s", version)

	// If version is "latest", check for newer image and pull if necessary
	if version == "latest" {
		log.Info("Checking for updates to the latest Docker image...")
		pullCmd := exec.Command("docker", "pull", imageName)
		pullOut, pullErr := pullCmd.CombinedOutput()
		pullOutput := string(pullOut)
		if pullErr != nil {
			log.Errorf("Failed to pull the latest image: %v", pullErr)
			log.Errorf("Pull command output: %s", pullOutput)
			return fmt.Errorf("failed to pull the latest image: %w\nOutput: %s", pullErr, pullOutput)
		}

		// Check if the image was updated or is up-to-date
		if strings.Contains(pullOutput, "Downloaded newer image") {
			log.Infof("Docker image updated to the latest version.")
		} else if strings.Contains(pullOutput, "Image is up to date") {
			log.Infof("Docker image is already the latest version.")
		} else {
			log.Warnf("Unexpected output from docker pull command: %s", pullOutput)
		}
	}

	cmdArgs := []string{
		"docker", "run", "-it",
		"-v", fmt.Sprintf("%s:/data", c.TopoPaths.TopologyFileDir()),
		imageName,
		"-i", topoFile,
	}

	log.Infof("Generating draw.io diagram with version: %s", version)

	// Process additional flags
	for _, flag := range additionalFlags {
		parts := strings.Fields(flag)
		cmdArgs = append(cmdArgs, parts...)
	}

	// Create the command
	cmd := exec.Command("sudo", cmdArgs...)

	// Start the command with a pseudo-terminal (PTY)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Errorf("Failed to start command with PTY: %v", err)
		return fmt.Errorf("failed to start command with PTY: %w", err)
	}
	defer func() { _ = ptmx.Close() }() // Best effort to close the PTY

	// Check if os.Stdin is a terminal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		// Handle PTY size changes
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		go func() {
			for range ch {
				if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
					log.Errorf("Error resizing PTY: %v", err)
				}
			}
		}()
		ch <- syscall.SIGWINCH // Initial resize

		// Set the terminal to raw mode
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			log.Errorf("Failed to set terminal to raw mode: %v", err)
			return fmt.Errorf("failed to set terminal to raw mode: %w", err)
		}
		defer func() {
			_ = term.Restore(int(os.Stdin.Fd()), oldState) // Best effort to restore
		}()

		// Copy stdin to the PTY and the PTY to stdout
		go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	}

	// Always copy the PTY output to our program's stdout
	// This ensures we capture the output regardless of TTY status
	_, _ = io.Copy(os.Stdout, ptmx)

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		log.Errorf("Command execution failed: %v", err)
		return fmt.Errorf("failed to generate diagram: %w", err)
	}

	log.Infof("Diagram created successfully.")

	return nil
}
