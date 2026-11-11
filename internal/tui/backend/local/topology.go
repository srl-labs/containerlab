// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	// topologyDataFile is the export of the deployed topology in the lab directory.
	topologyDataFile = "topology-data.json"
	// annotationsSuffix is appended to the name of a topology file to get the file the
	// containerlab web application keeps its drawing of the topology in.
	annotationsSuffix = ".annotations.json"

	// maxTopologyFileSize bounds the files read to learn about a topology.
	maxTopologyFileSize = 16 << 20
	// maxCachedTopologies bounds the cache of topologies.
	maxCachedTopologies = 64

	// Node labels that describe the drawing of a topology, for topologies without a
	// drawing file.
	labelGraphLevel = "graph-level"
	labelGraphIcon  = "graph-icon"
	labelGraphGroup = "graph-group"
	labelViewerRole = "topoViewer-role"
)

// topologyData is the part of the topology export the backend reads.
type topologyData struct {
	Name  string                  `json:"name"`
	Nodes map[string]topoDataNode `json:"nodes"`
	Links []topoDataLink          `json:"links"`
}

type topoDataNode struct {
	Index        flexString        `json:"index"`
	ShortName    string            `json:"shortname"`
	LongName     string            `json:"longname"`
	Fqdn         string            `json:"fqdn"`
	Group        string            `json:"group"`
	LabDir       string            `json:"labdir"`
	Kind         string            `json:"kind"`
	Image        string            `json:"image"`
	MgmtNet      string            `json:"mgmt-net"`
	MgmtIntf     string            `json:"mgmt-intf"`
	MgmtIPv4     string            `json:"mgmt-ipv4-address"`
	MgmtIPv4Len  int               `json:"mgmt-ipv4-prefix-length"`
	MgmtIPv6     string            `json:"mgmt-ipv6-address"`
	MgmtIPv6Len  int               `json:"mgmt-ipv6-prefix-length"`
	MAC          string            `json:"mac-address"`
	Labels       map[string]string `json:"labels"`
	PortBindings []topoDataPort    `json:"port-bindings"`
}

type topoDataPort struct {
	HostIP   string `json:"host-ip"`
	HostPort int    `json:"host-port"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type topoDataLink struct {
	Endpoints struct {
		A *topoDataEndpoint `json:"a"`
		Z *topoDataEndpoint `json:"z"`
	} `json:"endpoints"`
}

type topoDataEndpoint struct {
	Node      string `json:"node"`
	Interface string `json:"interface"`
	MAC       string `json:"mac"`
}

// annotations is the part of a drawing file the backend reads.
type annotations struct {
	Nodes []nodeAnnotation `json:"nodeAnnotations"`
}

type nodeAnnotation struct {
	ID       string `json:"id"`
	Position *struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"position"`
	Icon  string     `json:"icon"`
	Level flexString `json:"level"`
	// Group is the legacy way to name the group of a node, GroupID the current one.
	Group   string `json:"group"`
	GroupID string `json:"groupId"`
}

// node returns the annotation of a node, nil when there is none.
func (a *annotations) node(name string) *nodeAnnotation {
	if a == nil {
		return nil
	}

	for i := range a.Nodes {
		if a.Nodes[i].ID == name {
			return &a.Nodes[i]
		}
	}

	return nil
}

// flexString is a string that is also read from a JSON number. The files it is used for
// are written by other programs, some of which do not quote numbers.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexString(s)

		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexString(n.String())
	}

	// A value of any other type is as good as none.
	return nil
}

// readLimited reads a file that is not expected to be larger than maxTopologyFileSize.
func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxTopologyFileSize+1))
	if err != nil {
		return nil, err
	}

	if len(data) > maxTopologyFileSize {
		return nil, fmt.Errorf("%s is larger than %d bytes", path, maxTopologyFileSize)
	}

	return data, nil
}

func readTopologyData(path string) (*topologyData, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}

	var topo topologyData
	if err := json.Unmarshal(data, &topo); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &topo, nil
}

// readAnnotations reads a drawing file. The file is optional and never written: nil is
// returned when it is missing or unreadable.
func readAnnotations(path string) *annotations {
	data, err := readLimited(path)
	if err != nil {
		return nil
	}

	var ann annotations
	if err := json.Unmarshal(data, &ann); err != nil {
		return nil
	}

	return &ann
}

// placeNode fills in what a drawing tells about a node: its position, role, tier and
// group. The drawing file wins over the labels of the node.
func placeNode(
	node *clabinternaltuibackend.TopoNode,
	labels map[string]string,
	ann *nodeAnnotation,
) {
	role := cmp.Or(labels[labelGraphIcon], labels[labelViewerRole])
	level := labels[labelGraphLevel]
	group := labels[labelGraphGroup]

	if ann != nil {
		role = cmp.Or(ann.Icon, role)
		level = cmp.Or(string(ann.Level), level)
		group = cmp.Or(ann.Group, ann.GroupID, group)

		if ann.Position != nil {
			node.X, node.Y, node.HasPos = ann.Position.X, ann.Position.Y, true
		}
	}

	node.Role = role
	node.Level = parseLevel(level)
	node.Group = cmp.Or(group, node.Group)
}

// parseLevel reads the tier of a node, which is written as a number or as a number with a
// prefix, ie. "L2".
func parseLevel(s string) int {
	start := strings.IndexFunc(s, func(r rune) bool { return r >= '0' && r <= '9' })
	if start < 0 {
		return 0
	}

	level, _ := strconv.Atoi(digits(s[start:]))

	return level
}

// topologyFromData builds the topology of a deployed lab from its export.
func topologyFromData(
	lab string,
	data *topologyData,
	ann *annotations,
) clabinternaltuibackend.Topology {
	topo := clabinternaltuibackend.Topology{Lab: lab}

	for _, name := range sortedNames(maps.Keys(data.Nodes)) {
		exported := data.Nodes[name]
		node := clabinternaltuibackend.TopoNode{
			Name:  name,
			Kind:  exported.Kind,
			Group: exported.Group,
		}

		placeNode(&node, exported.Labels, ann.node(name))
		topo.Nodes = append(topo.Nodes, node)
	}

	endpoint := func(ep *topoDataEndpoint) clabinternaltuibackend.Endpoint {
		_, isNode := data.Nodes[ep.Node]

		return clabinternaltuibackend.Endpoint{
			Node:      ep.Node,
			Interface: ep.Interface,
			MAC:       ep.MAC,
			External:  !isNode,
		}
	}

	for i := range data.Links {
		ends := &data.Links[i].Endpoints
		if ends.A == nil {
			continue
		}

		link := clabinternaltuibackend.Link{A: endpoint(ends.A)}

		if ends.Z != nil {
			z := endpoint(ends.Z)
			link.Z = &z
		}

		topo.Links = append(topo.Links, link)
	}

	return topo
}

// topologyFromCLab builds the topology of a parsed topology file.
func topologyFromCLab(
	lab string,
	c *clabcore.CLab,
	ann *annotations,
) clabinternaltuibackend.Topology {
	topo := clabinternaltuibackend.Topology{Lab: lab}

	for _, name := range sortedNames(maps.Keys(c.Nodes)) {
		cfg := c.Nodes[name].Config()
		if cfg == nil || cfg.Labels[clabconstants.InternalNode] == "true" {
			continue
		}

		node := clabinternaltuibackend.TopoNode{Name: name, Kind: cfg.Kind, Group: cfg.Group}

		placeNode(&node, cfg.Labels, ann.node(name))
		topo.Nodes = append(topo.Nodes, node)
	}

	if err := c.ResolveLinks(); err != nil {
		topo.Warning = "failed to resolve the links: " + oneLine(err.Error())

		return topo
	}

	// The MAC addresses of a topology that is not deployed are made up while parsing and
	// would differ from the ones a deployment gets, so they are left out.
	endpoint := func(ep clablinks.Endpoint) clabinternaltuibackend.Endpoint {
		name := ""
		if node := ep.GetNode(); node != nil {
			name = node.GetShortName()
		}

		_, isNode := c.Nodes[name]

		return clabinternaltuibackend.Endpoint{
			Node:      name,
			Interface: ep.GetIfaceName(),
			External:  !isNode || ep.IsNodeless(),
		}
	}

	for _, id := range slices.Sorted(maps.Keys(c.Links)) {
		ends := c.Links[id].GetEndpoints()
		if len(ends) == 0 {
			continue
		}

		link := clabinternaltuibackend.Link{A: endpoint(ends[0])}

		if len(ends) > 1 {
			z := endpoint(ends[1])
			link.Z = &z
		}

		topo.Links = append(topo.Links, link)
	}

	return topo
}

// topologyFromNodes builds a topology without links from the nodes of a deployed lab.
func topologyFromNodes(
	lab *clabinternaltuibackend.Lab,
	ann *annotations,
) clabinternaltuibackend.Topology {
	topo := clabinternaltuibackend.Topology{Lab: lab.Name}

	for i := range lab.Nodes {
		n := &lab.Nodes[i]
		node := clabinternaltuibackend.TopoNode{Name: n.Name, Kind: n.Kind, Group: n.Group}

		placeNode(&node, n.Labels, ann.node(n.Name))
		topo.Nodes = append(topo.Nodes, node)
	}

	return topo
}

func sortedNames(names iter.Seq[string]) []string {
	return slices.SortedFunc(names, clabinternaltuitermtext.NaturalCompare)
}

// oneLine makes a text fit on a line of the UI.
func oneLine(s string) string {
	return clabinternaltuitermtext.Sanitize(strings.Join(strings.Fields(s), " "))
}

func cloneTopology(topo *clabinternaltuibackend.Topology) clabinternaltuibackend.Topology {
	out := *topo
	out.Nodes = slices.Clone(topo.Nodes)
	out.Links = slices.Clone(topo.Links)

	for i := range out.Links {
		if out.Links[i].Z != nil {
			z := *out.Links[i].Z
			out.Links[i].Z = &z
		}
	}

	return out
}

// fileStamp identifies a version of a file.
type fileStamp struct {
	exists  bool
	modTime time.Time
	size    int64
}

func stampOf(path string) fileStamp {
	if path == "" {
		return fileStamp{}
	}

	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}

	return fileStamp{exists: true, modTime: info.ModTime(), size: info.Size()}
}

// topoEntry is a cached topology with the versions of the files it was built from.
type topoEntry struct {
	stamps []fileStamp
	topo   clabinternaltuibackend.Topology
}

// topologies caches the topologies of the labs. An entry is valid for as long as the files
// it was built from do not change.
type topologies struct {
	mu      sync.Mutex
	entries map[string]*topoEntry
	// loading holds the keys of the topology files that are being parsed.
	loading map[string]bool
}

// get returns the cached topology for a key. current tells whether it was built from the
// given versions of its files; a topology built from older ones is returned as well.
func (t *topologies) get(
	key string,
	stamps []fileStamp,
) (topo clabinternaltuibackend.Topology, found, current bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry := t.entries[key]
	if entry == nil {
		return topo, false, false
	}

	return entry.topo, true, slices.Equal(entry.stamps, stamps)
}

func (t *topologies) put(key string, stamps []fileStamp, topo *clabinternaltuibackend.Topology) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.entries == nil || len(t.entries) >= maxCachedTopologies {
		t.entries = map[string]*topoEntry{}
	}

	t.entries[key] = &topoEntry{stamps: stamps, topo: *topo}
}

// startLoading marks a key as being loaded. It fails when it is marked already.
func (t *topologies) startLoading(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.loading[key] {
		return false
	}

	if t.loading == nil {
		t.loading = map[string]bool{}
	}

	t.loading[key] = true

	return true
}

func (t *topologies) doneLoading(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.loading, key)
}

// Topology returns the nodes and links of a lab. For a lab that is not deployed the
// topology file has to be parsed first, which happens in the background: until that is
// done the topology comes with a warning and no content, and the links topic is signaled
// once it is available.
func (b *Backend) Topology(lab string) clabinternaltuibackend.Topology {
	if rec := b.inv.record(lab); rec != nil {
		topo := b.deployedTopology(rec)

		return cloneTopology(&topo)
	}

	if file, ok := b.files.byName(lab); ok {
		topo := b.fileTopology(lab, file.path)

		return cloneTopology(&topo)
	}

	return clabinternaltuibackend.Topology{Lab: lab, Warning: "unknown lab"}
}

// deployedTopology returns the topology of a deployed lab, read from the export in its lab
// directory. The result shares memory with the cache and must not be changed.
func (b *Backend) deployedTopology(rec *labRecord) clabinternaltuibackend.Topology {
	lab := &rec.lab

	// A lab in a cluster has no lab directory with an export on this host.
	if rec.namespace != "" {
		return b.labRuntimeTopology(rec)
	}

	var dataPath, annPath string

	if lab.LabDir != "" {
		dataPath = filepath.Join(lab.LabDir, topologyDataFile)
	}

	if lab.TopoFile != "" {
		annPath = lab.TopoFile + annotationsSuffix
	}

	key := "lab:" + lab.Name
	stamps := []fileStamp{stampOf(dataPath), stampOf(annPath)}

	if topo, _, current := b.topo.get(key, stamps); current {
		return topo
	}

	data, err := readTopologyData(dataPath)
	if err == nil && len(data.Nodes) == 0 {
		err = errors.New("the topology export of the lab lists no nodes")
	}

	if err != nil {
		// Without the export the topology file still tells how the lab is wired, and
		// without that file the nodes at least are known.
		if !lab.TopoMissing {
			return b.fileTopology(lab.Name, lab.TopoFile)
		}

		topo := topologyFromNodes(lab, readAnnotations(annPath))
		topo.Warning = "the links are unknown: " + oneLine(err.Error())

		return topo
	}

	topo := topologyFromData(lab.Name, data, readAnnotations(annPath))
	b.topo.put(key, stamps, &topo)

	return topo
}

// fileTopology returns the topology of a topology file. The file is parsed in the
// background when it was not parsed yet or changed since; until then the previous result,
// or a placeholder, is returned.
func (b *Backend) fileTopology(lab, file string) clabinternaltuibackend.Topology {
	key := "file:" + file
	stamps := []fileStamp{stampOf(file), stampOf(file + annotationsSuffix)}

	topo, found, current := b.topo.get(key, stamps)
	if current {
		return topo
	}

	if b.topo.startLoading(key) {
		b.spawn("topology parser", func() { b.parseTopology(key, lab, file, stamps) })
	}

	if found {
		return topo
	}

	return clabinternaltuibackend.Topology{Lab: lab, Warning: "the topology is being read"}
}

// parseTopology parses a topology file and caches the result.
func (b *Backend) parseTopology(key, lab, file string, stamps []fileStamp) {
	// Deferred calls run in reverse order: the UI is told only once a new parse can start.
	defer b.mark(clabinternaltuibackend.TopicLinks)
	defer b.topo.doneLoading(key)

	// Parsing is heavy enough to do one file at a time.
	b.parseMu.Lock()
	defer b.parseMu.Unlock()

	topo := clabinternaltuibackend.Topology{Lab: lab}

	if c, err := b.loadTopo(file); err != nil {
		topo.Warning = "failed to read the topology: " + oneLine(err.Error())
	} else {
		topo = topologyFromCLab(lab, c, readAnnotations(file+annotationsSuffix))
	}

	b.topo.put(key, stamps, &topo)
}

// loadTopologyFile parses a topology file the way the containerlab commands do. Bind paths
// are not checked, they only matter once the lab is deployed.
func (b *Backend) loadTopologyFile(file string) (*clabcore.CLab, error) {
	c, err := clabcore.NewContainerLab(
		clabcore.WithTimeout(b.opts.Timeout),
		clabcore.WithRuntime(b.rtName, &clabruntime.RuntimeConfig{
			Timeout:      b.opts.Timeout,
			Debug:        b.opts.Debug,
			LabNamespace: b.opts.Namespace,
		}),
		clabcore.WithSkippedBindsPathsCheck(),
		clabcore.WithTopoPath(file, nil),
	)
	if err != nil {
		return nil, err
	}

	// For a lab runtime the nodes and links of a topology are only built on request.
	if err := c.LoadTopologyNodes(); err != nil {
		return nil, err
	}

	return c, nil
}
