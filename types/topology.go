package types

import (
	"strings"

	"github.com/docker/go-connections/nat"
	clablinks "github.com/srl-labs/containerlab/links"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func getField[T any](
	topo *Topology,
	nodeName string,
	getFieldNode func(*NodeDefinition) T,
	getFieldGroup func(*NodeDefinition) T,
	getFieldKind func(*NodeDefinition) T,
	getFieldDefaults func(*NodeDefinition) T,
	isSet func(T) bool,
) T {
	fieldDefault := getFieldDefaults(topo.GetDefaults())

	nodeDefintion, ok := topo.Nodes[nodeName]
	if !ok {
		return fieldDefault
	}

	fieldNode := getFieldNode(nodeDefintion)
	if isSet(fieldNode) {
		return fieldNode
	}

	group := topo.GetGroup(topo.GetNodeGroup(nodeName))
	if group != nil {
		fieldGroup := getFieldGroup(group)
		if isSet(fieldGroup) {
			return fieldGroup
		}
	}

	kind := topo.GetKind(topo.GetNodeKind(nodeName))
	if kind != nil {
		fieldKind := getFieldKind(kind)
		if isSet(fieldKind) {
			return fieldKind
		}
	}

	return fieldDefault
}

// a pointer to T -- for generically handling *bool or similar type fields we are trying to unpack
// from a node definition.
type ptrTo[T any] interface {
	~*T
}

func getFieldPtr[T any, P ptrTo[T]](
	topo *Topology,
	nodeName string,
	getFieldNode func(*NodeDefinition) P,
	getFieldGroup func(*NodeDefinition) P,
	getFieldKind func(*NodeDefinition) P,
	getFieldDefaults func(*NodeDefinition) P,
	isSet func(P) bool,
	lastRestortDefault T,
) T {
	fieldDefault := getFieldDefaults(topo.GetDefaults())

	finalDefault := lastRestortDefault
	if fieldDefault != nil {
		finalDefault = *fieldDefault
	}

	nodeDefintion, ok := topo.Nodes[nodeName]
	if !ok {
		return finalDefault
	}

	fieldNode := getFieldNode(nodeDefintion)
	if isSet(fieldNode) {
		return *fieldNode
	}

	group := topo.GetGroup(topo.GetNodeGroup(nodeName))
	if group != nil {
		fieldGroup := getFieldGroup(group)
		if isSet(fieldGroup) {
			return *fieldGroup
		}
	}

	kind := topo.GetKind(topo.GetNodeKind(nodeName))
	if kind != nil {
		fieldKind := getFieldKind(kind)
		if isSet(fieldKind) {
			return *fieldKind
		}
	}

	return finalDefault
}

// Topology represents a lab topology.
type Topology struct {
	Defaults *NodeDefinition             `yaml:"defaults,omitempty"`
	Kinds    map[string]*NodeDefinition  `yaml:"kinds,omitempty"`
	Nodes    map[string]*NodeDefinition  `yaml:"nodes,omitempty"`
	Groups   map[string]*NodeDefinition  `yaml:"groups,omitempty"`
	Links    []*clablinks.LinkDefinition `yaml:"links,omitempty"`
}

// NewTopology creates a new Topology instance with initialized fields.
func NewTopology() *Topology {
	return &Topology{
		Defaults: new(NodeDefinition),
		Kinds:    make(map[string]*NodeDefinition),
		Nodes:    make(map[string]*NodeDefinition),
		Groups:   make(map[string]*NodeDefinition),
		Links:    make([]*clablinks.LinkDefinition, 0),
	}
}

// GetDefaults returns the default node definition.
func (t *Topology) GetDefaults() *NodeDefinition {
	if t.Defaults != nil {
		return t.Defaults
	}

	return new(NodeDefinition)
}

// GetKind returns the node definition for the given kind.
func (t *Topology) GetKind(kind string) *NodeDefinition {
	if t.Kinds == nil {
		return new(NodeDefinition)
	}

	if kdef, ok := t.Kinds[kind]; ok {
		return kdef
	}

	return new(NodeDefinition)
}

// GetKinds returns all kinds defined in the topology.
// If no kinds are defined, it returns an empty map.
func (t *Topology) GetKinds() map[string]*NodeDefinition {
	if t.Kinds == nil {
		return make(map[string]*NodeDefinition)
	}

	return t.Kinds
}

func (t *Topology) GetGroup(group string) *NodeDefinition {
	if t.Groups == nil {
		return nil
	}

	if gdef, ok := t.Groups[group]; ok {
		return gdef
	}

	return nil
}

func (t *Topology) GetGroups() map[string]*NodeDefinition {
	if t.Groups == nil {
		return make(map[string]*NodeDefinition)
	}

	return t.Groups
}

func (t *Topology) GetNodeKind(name string) string {
	defaultKind := t.GetDefaults().Kind

	nodeDefinition, ok := t.Nodes[name]
	if !ok {
		// if no node kind is set, there is no way for us to look up the kind in the kind
		return defaultKind
	}

	kind := nodeDefinition.Kind

	if kind != "" {
		return kind
	}

	group := t.GetGroup(nodeDefinition.Group)

	if group != nil {
		// Check if the node actually has a group, then get the kind from groups
		groupKind := group.Kind

		if groupKind != "" {
			return groupKind
		}
	}

	defaultGroup := t.GetGroup(t.Defaults.Group)

	if defaultGroup != nil {
		// Check for groups at default levels
		defaultsGroupKind := defaultGroup.Kind

		if defaultsGroupKind != "" {
			return defaultsGroupKind
		}
	}

	return defaultKind
}

func (t *Topology) GetNodeBinds(name string) ([]string, error) {
	if _, ok := t.Nodes[name]; !ok {
		return nil, nil
	}

	binds := map[string]*Bind{}

	// group the default, kind, group and node binds
	bindSources := [][]string{
		t.GetDefaults().GetBinds(),
		t.GetKind(t.GetNodeKind(name)).GetBinds(),
		t.GetGroup(t.GetNodeGroup(name)).GetBinds(),
		t.Nodes[name].GetBinds(),
	}

	// add the binds from less to more specific levels, indexed by the destination path.
	// thereby more specific binds will overwrite less specific one
	for _, bs := range bindSources {
		for _, bind := range bs {
			b, err := NewBindFromString(bind)
			if err != nil {
				return nil, err
			}

			binds[b.Dst()] = b
		}
	}

	// in order to return nil instead of empty array when no binds are defined
	if len(binds) == 0 {
		return nil, nil
	}

	// build the result array with all the entries from binds map
	result := make([]string, 0, len(binds))

	for _, b := range binds {
		result = append(result, b.String())
	}

	return result, nil
}

func (t *Topology) GetNodePorts(name string) (nat.PortSet, nat.PortMap, error) {
	if ndef, ok := t.Nodes[name]; ok {
		// node level ports
		if len(ndef.GetPorts()) != 0 {
			return nat.ParsePortSpecs(ndef.Ports)
		}

		// group level ports
		if len(t.GetGroup(t.GetNodeGroup(name)).GetPorts()) > 0 {
			return nat.ParsePortSpecs(t.GetGroup(t.GetNodeGroup(name)).GetPorts())
		}

		// kind level ports
		if len(t.GetKind(t.GetNodeKind(name)).GetPorts()) > 0 {
			return nat.ParsePortSpecs(t.GetKind(t.GetNodeKind(name)).GetPorts())
		}

		// default level ports
		if len(t.GetDefaults().GetPorts()) > 0 {
			return nat.ParsePortSpecs(t.GetDefaults().GetPorts())
		}
	}

	return nil, nil, nil
}

func (t *Topology) GetNodeEnv(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return clabutils.MergeStringMaps(
			clabutils.MergeStringMaps(t.GetDefaults().GetEnv(),
				t.GetKind(t.GetNodeKind(name)).GetEnv(),
				t.GetGroup(t.GetNodeGroup(name)).GetEnv()),
			ndef.GetEnv())
	}

	return nil
}

func (t *Topology) GetNodeEnvFiles(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		return clabutils.MergeStringSlices(
			clabutils.MergeStringSlices(t.GetDefaults().GetEnvFiles(),
				t.GetKind(t.GetNodeKind(name)).GetEnvFiles(),
				t.GetGroup(t.GetNodeGroup(name)).GetEnvFiles()),
			ndef.GetEnvFiles())
	}

	return nil
}

func (t *Topology) GetNodeLabels(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return clabutils.MergeStringMaps(t.Defaults.GetLabels(),
			t.GetKind(t.GetNodeKind(name)).GetLabels(),
			t.GetGroup(t.GetNodeGroup(name)).GetLabels(),
			ndef.GetLabels())
	}

	return nil
}

func (t *Topology) GetNodeConfigDispatcher(name string) *ConfigDispatcher {
	if ndef, ok := t.Nodes[name]; ok {
		vars := clabutils.MergeMaps(t.Defaults.GetConfigDispatcher().GetVars(),
			t.GetKind(t.GetNodeKind(name)).GetConfigDispatcher().GetVars(),
			t.GetGroup(t.GetNodeGroup(name)).GetConfigDispatcher().GetVars(),
			ndef.GetConfigDispatcher().GetVars())

		return &ConfigDispatcher{
			Vars: vars,
		}
	}

	return nil
}

func (t *Topology) GetNodeDevices(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		return clabutils.MergeStringSlices(
			clabutils.MergeStringSlices(t.GetDefaults().GetDevices(),
				t.GetKind(t.GetNodeKind(name)).GetDevices(),
				t.GetGroup(t.GetNodeGroup(name)).GetDevices()),
			ndef.GetDevices())
	}

	return nil
}

func (t *Topology) GetNodeCapAdd(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		return clabutils.MergeStringSlices(
			clabutils.MergeStringSlices(t.GetDefaults().GetCapAdd(),
				t.GetKind(t.GetNodeKind(name)).GetCapAdd(),
				t.GetGroup(t.GetNodeGroup(name)).GetCapAdd()),
			ndef.GetCapAdd())
	}

	return nil
}

func (t *Topology) GetNodeShmSize(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.ShmSize },
		func(group *NodeDefinition) string { return group.ShmSize },
		func(kind *NodeDefinition) string { return kind.ShmSize },
		func(defaults *NodeDefinition) string { return defaults.ShmSize },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetComponents(name string) []*Component {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetComponents(); v != nil {
			return v
		}

		if v := t.GetGroup(t.GetNodeGroup(name)).GetComponents(); v != nil {
			return v
		}

		if v := t.GetKind(t.GetNodeKind(name)).GetComponents(); v != nil {
			return v
		}
	}

	return t.GetDefaults().GetComponents()
}

func (t *Topology) GetNodeStartupConfig(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.StartupConfig },
		func(group *NodeDefinition) string { return group.StartupConfig },
		func(kind *NodeDefinition) string { return kind.StartupConfig },
		func(defaults *NodeDefinition) string { return defaults.StartupConfig },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeStartupDelay(nodeName string) uint {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) uint { return node.StartupDelay },
		func(group *NodeDefinition) uint { return group.StartupDelay },
		func(kind *NodeDefinition) uint { return kind.StartupDelay },
		func(defaults *NodeDefinition) uint { return defaults.StartupDelay },
		func(v uint) bool { return v != 0 },
	)
}

func (t *Topology) GetNodeEnforceStartupConfig(nodeName string) bool {
	return getFieldPtr(
		t,
		nodeName,
		func(node *NodeDefinition) *bool { return node.EnforceStartupConfig },
		func(group *NodeDefinition) *bool { return group.EnforceStartupConfig },
		func(kind *NodeDefinition) *bool { return kind.EnforceStartupConfig },
		func(defaults *NodeDefinition) *bool { return defaults.EnforceStartupConfig },
		func(v *bool) bool { return v != nil },
		false,
	)
}

func (t *Topology) GetNodeSuppressStartupConfig(nodeName string) bool {
	return getFieldPtr(
		t,
		nodeName,
		func(node *NodeDefinition) *bool { return node.SuppressStartupConfig },
		func(group *NodeDefinition) *bool { return group.SuppressStartupConfig },
		func(kind *NodeDefinition) *bool { return kind.SuppressStartupConfig },
		func(defaults *NodeDefinition) *bool { return defaults.SuppressStartupConfig },
		func(v *bool) bool { return v != nil },
		false,
	)
}

func (t *Topology) GetNodeAutoRemove(nodeName string) bool {
	return getFieldPtr(
		t,
		nodeName,
		func(node *NodeDefinition) *bool { return node.AutoRemove },
		func(group *NodeDefinition) *bool { return group.AutoRemove },
		func(kind *NodeDefinition) *bool { return kind.AutoRemove },
		func(defaults *NodeDefinition) *bool { return defaults.AutoRemove },
		func(v *bool) bool { return v != nil },
		false,
	)
}

func (t *Topology) GetRestartPolicy(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.RestartPolicy },
		func(group *NodeDefinition) string { return group.RestartPolicy },
		func(kind *NodeDefinition) string { return kind.RestartPolicy },
		func(defaults *NodeDefinition) string { return defaults.RestartPolicy },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeLicense(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.License },
		func(group *NodeDefinition) string { return group.License },
		func(kind *NodeDefinition) string { return kind.License },
		func(defaults *NodeDefinition) string { return defaults.License },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeImage(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.Image },
		func(group *NodeDefinition) string { return group.Image },
		func(kind *NodeDefinition) string { return kind.Image },
		func(defaults *NodeDefinition) string { return defaults.Image },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeImagePullPolicy(name string) PullPolicyValue {
	if ndef, ok := t.Nodes[name]; ok {
		if pp := ndef.GetImagePullPolicy(); pp != "" {
			return ParsePullPolicyValue(pp)
		}

		if pp := t.GetGroup(t.GetNodeGroup(name)).GetImagePullPolicy(); pp != "" {
			return ParsePullPolicyValue(pp)
		}

		if pp := t.GetKind(t.GetNodeKind(name)).GetImagePullPolicy(); pp != "" {
			return ParsePullPolicyValue(pp)
		}
	}

	return ParsePullPolicyValue(t.GetDefaults().GetImagePullPolicy())
}

func (t *Topology) GetNodeGroup(name string) string {
	defaultGroup := t.GetDefaults().Group

	nodeDefinition, ok := t.Nodes[name]
	if !ok {
		return defaultGroup
	}

	group := nodeDefinition.Group

	if group != "" {
		return group
	}

	kind := t.GetNodeKind(name)

	if kind != "" {
		// Check if the node actually has a kind, then get the group from kinds
		kindGroup := t.GetKind(kind).Group

		if kindGroup != "" {
			return kindGroup
		}
	}

	return defaultGroup
}

func (t *Topology) GetNodeType(name string) string {
	defaultType := t.GetDefaults().Type

	nodeDefinition, ok := t.Nodes[name]
	if !ok {
		return defaultType
	}

	if nodeDefinition.Type != "" {
		return strings.TrimSpace(nodeDefinition.Type)
	}

	group := t.GetGroup(t.GetNodeGroup(name))

	if group != nil && group.Type != "" {
		return strings.TrimSpace(group.Type)
	}

	kind := t.GetKind(t.GetNodeKind(name))

	if kind != nil && kind.Type != "" {
		return strings.TrimSpace(kind.Type)
	}

	return defaultType
}

func (t *Topology) GetNodePosition(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.Position },
		func(group *NodeDefinition) string { return group.Position },
		func(kind *NodeDefinition) string { return kind.Position },
		func(defaults *NodeDefinition) string { return defaults.Position },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeEntrypoint(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.Entrypoint },
		func(group *NodeDefinition) string { return group.Entrypoint },
		func(kind *NodeDefinition) string { return kind.Entrypoint },
		func(defaults *NodeDefinition) string { return defaults.Entrypoint },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeCmd(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.Cmd },
		func(group *NodeDefinition) string { return group.Cmd },
		func(kind *NodeDefinition) string { return kind.Cmd },
		func(defaults *NodeDefinition) string { return defaults.Cmd },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeExec(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		d := t.GetDefaults().GetExec()
		k := t.GetKind(t.GetNodeKind(name)).GetExec()
		g := t.GetGroup(t.GetNodeGroup(name)).GetExec()
		n := ndef.GetExec()

		return append(append(append(d, k...), g...), n...)
	}

	return nil
}

func (t *Topology) GetNodeUser(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.User },
		func(group *NodeDefinition) string { return group.User },
		func(kind *NodeDefinition) string { return kind.User },
		func(defaults *NodeDefinition) string { return defaults.User },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeNetworkMode(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.NetworkMode },
		func(group *NodeDefinition) string { return group.NetworkMode },
		func(kind *NodeDefinition) string { return kind.NetworkMode },
		func(defaults *NodeDefinition) string { return defaults.NetworkMode },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeSandbox(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.Sandbox },
		func(group *NodeDefinition) string { return group.Sandbox },
		func(kind *NodeDefinition) string { return kind.Sandbox },
		func(defaults *NodeDefinition) string { return defaults.Sandbox },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeKernel(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.Kernel },
		func(group *NodeDefinition) string { return group.Kernel },
		func(kind *NodeDefinition) string { return kind.Kernel },
		func(defaults *NodeDefinition) string { return defaults.Kernel },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeRuntime(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.Runtime },
		func(group *NodeDefinition) string { return group.Runtime },
		func(kind *NodeDefinition) string { return kind.Runtime },
		func(defaults *NodeDefinition) string { return defaults.Runtime },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeCPU(nodeName string) float64 {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) float64 { return node.CPU },
		func(group *NodeDefinition) float64 { return group.CPU },
		func(kind *NodeDefinition) float64 { return kind.CPU },
		func(defaults *NodeDefinition) float64 { return defaults.CPU },
		func(v float64) bool { return v != 0 },
	)
}

func (t *Topology) GetNodeCPUSet(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.CPUSet },
		func(group *NodeDefinition) string { return group.CPUSet },
		func(kind *NodeDefinition) string { return kind.CPUSet },
		func(defaults *NodeDefinition) string { return defaults.CPUSet },
		func(v string) bool { return v != "" },
	)
}

func (t *Topology) GetNodeMemory(nodeName string) string {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) string { return node.Memory },
		func(group *NodeDefinition) string { return group.Memory },
		func(kind *NodeDefinition) string { return kind.Memory },
		func(defaults *NodeDefinition) string { return defaults.Memory },
		func(v string) bool { return v != "" },
	)
}

// GetSysCtl return the Sysctl configuration for the given node.
func (t *Topology) GetSysCtl(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return clabutils.MergeStringMaps(
			clabutils.MergeStringMaps(t.GetDefaults().GetSysctls(),
				t.GetKind(t.GetNodeKind(name)).GetSysctls(),
				t.GetGroup(t.GetNodeGroup(name)).GetSysctls()),
			ndef.GetSysctls())
	}

	return nil
}

// GetNodeExtras returns the 'extras' section for the given node.
func (t *Topology) GetNodeExtras(name string) *Extras {
	if ndef, ok := t.Nodes[name]; ok {
		node_extras := ndef.GetExtras()
		if node_extras != nil {
			return node_extras
		}

		group_extras := t.GetGroup(t.GetNodeGroup(name)).GetExtras()
		if group_extras != nil {
			return group_extras
		}

		kind_extras := t.GetKind(t.GetNodeKind(name)).GetExtras()
		if kind_extras != nil {
			return kind_extras
		}
	}

	return t.GetDefaults().GetExtras()
}

// GetStages return the configuration stages set for the given node.
// It merges the default, kind and node stages into a single Stages struct
// with node stages taking precedence over kind stages and default stages.
func (t *Topology) GetStages(name string) (*Stages, error) {
	s := NewStages()

	// default Stages
	defaultStages := t.GetDefaults().Stages
	if defaultStages != nil {
		s.Merge(defaultStages)
	}

	// kind Stages
	kindStages := t.GetKind(t.GetNodeKind(name)).GetStages()
	if kindStages != nil {
		s.Merge(kindStages)
	}

	// group Stages
	groupStages := t.GetGroup(t.GetNodeGroup(name)).GetStages()
	if groupStages != nil {
		s.Merge(groupStages)
	}

	// node Stages
	nodeStages := t.Nodes[name].GetStages()
	if nodeStages != nil {
		s.Merge(nodeStages)
	}

	// set nil values to their respective defaults
	s.InitDefaults()

	return s, nil
}

func (t *Topology) ImportEnvs() {
	t.Defaults.ImportEnvs()

	for _, k := range t.Kinds {
		k.ImportEnvs()
	}

	for _, g := range t.Groups {
		g.ImportEnvs()
	}

	for _, n := range t.Nodes {
		n.ImportEnvs()
	}
}

func (t *Topology) GetNodeDns(name string) *DNSConfig {
	if ndef, ok := t.Nodes[name]; ok {
		nodeDNS := ndef.GetDns()
		if nodeDNS != nil {
			return nodeDNS
		}

		groupDNS := t.GetGroup(t.GetNodeGroup(name)).GetDns()
		if groupDNS != nil {
			return groupDNS
		}

		kindDNS := t.GetKind(t.GetNodeKind(name)).GetDns()
		if kindDNS != nil {
			return kindDNS
		}
	}

	defaultDNS := t.GetDefaults().GetDns()
	if defaultDNS == nil {
		// initialize defaultDNS to an empty DNSConfig struct
		// so that we don't have to check for nil in the caller
		defaultDNS = &DNSConfig{}
	}

	return defaultDNS
}

// GetCertificateConfig returns the certificate configuration for the given node.
func (t *Topology) GetCertificateConfig(name string) *CertificateConfig {
	// default for issuing node certificates is false
	cc := &CertificateConfig{
		Issue: clabutils.Pointer(false),
	}

	// merge defaults, kind and node certificate config into the default certificate config
	cc.Merge(
		t.GetDefaults().GetCertificateConfig()).Merge(
		t.GetKind(t.GetNodeKind(name)).GetCertificateConfig()).Merge(
		t.GetGroup(t.GetNodeGroup(name)).GetCertificateConfig()).Merge(
		t.Nodes[name].GetCertificateConfig())

	return cc
}

func (t *Topology) GetHealthCheckConfig(name string) *HealthcheckConfig {
	if ndef, ok := t.Nodes[name]; ok {
		nodeHealthcheckConf := ndef.GetHealthcheckConfig()
		if nodeHealthcheckConf != nil {
			return nodeHealthcheckConf
		}

		groupHealthcheckConf := t.GetGroup(t.GetNodeGroup(name)).GetHealthcheckConfig()
		if groupHealthcheckConf != nil {
			return groupHealthcheckConf
		}

		kindHealthcheckConf := t.GetKind(t.GetNodeKind(name)).GetHealthcheckConfig()
		if kindHealthcheckConf != nil {
			return kindHealthcheckConf
		}

		return t.GetDefaults().GetHealthcheckConfig()
	}

	return nil
}

func (t *Topology) GetNodeAliases(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		return ndef.GetAliases()
	}

	return nil
}
