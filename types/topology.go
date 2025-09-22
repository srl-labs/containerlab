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
	if nodeDefintion == nil || !ok {
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
	if nodeDefintion == nil || !ok {
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

func mergeStringSliceFields(
	topo *Topology,
	nodeName string,
	getFieldNode func(*NodeDefinition) []string,
	getFieldGroup func(*NodeDefinition) []string,
	getFieldKind func(*NodeDefinition) []string,
	getFieldDefaults func(*NodeDefinition) []string,
) []string {
	var out []string

	nodeDefintion, ok := topo.Nodes[nodeName]
	if nodeDefintion == nil || !ok {
		return out
	}

	fieldNode := getFieldNode(nodeDefintion)

	var fieldGroup []string

	group := topo.GetGroup(topo.GetNodeGroup(nodeName))
	if group != nil {
		fieldGroup = getFieldGroup(group)
	}

	var fieldKind []string

	kind := topo.GetKind(topo.GetNodeKind(nodeName))
	if kind != nil {
		fieldKind = getFieldKind(kind)
	}

	defaultsField := getFieldDefaults(topo.GetDefaults())

	// merge "backwards" for prioritization reasons
	return clabutils.MergeStringSlices(defaultsField, fieldKind, fieldGroup, fieldNode)
}

func mergeStringMapFields(
	topo *Topology,
	nodeName string,
	getFieldNode func(*NodeDefinition) map[string]string,
	getFieldGroup func(*NodeDefinition) map[string]string,
	getFieldKind func(*NodeDefinition) map[string]string,
	getFieldDefaults func(*NodeDefinition) map[string]string,
) map[string]string {
	out := map[string]string{}

	nodeDefintion, ok := topo.Nodes[nodeName]
	if nodeDefintion == nil || !ok {
		return out
	}

	fieldNode := getFieldNode(nodeDefintion)

	var fieldGroup map[string]string

	group := topo.GetGroup(topo.GetNodeGroup(nodeName))
	if group != nil {
		fieldGroup = getFieldGroup(group)
	}

	var fieldKind map[string]string

	kind := topo.GetKind(topo.GetNodeKind(nodeName))
	if kind != nil {
		fieldKind = getFieldKind(kind)
	}

	defaultsField := getFieldDefaults(topo.GetDefaults())

	// merge "backwards" for prioritization reasons
	mergedOut := clabutils.MergeStringMaps(defaultsField, fieldKind, fieldGroup, fieldNode)
	if mergedOut == nil {
		return out
	}

	return mergedOut
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

func (t *Topology) GetNodeKind(nodeName string) string {
	defaultKind := t.GetDefaults().Kind

	nodeDefinition, ok := t.Nodes[nodeName]
	if nodeDefinition == nil || !ok {
		return defaultKind
	}

	if nodeDefinition.Kind != "" {
		return nodeDefinition.Kind
	}

	group := t.GetGroup(nodeDefinition.Group)
	if group != nil && group.Kind != "" {
		return group.Kind
	}

	defaults := t.GetGroup(t.Defaults.Group)
	if defaults != nil && defaults.Kind != "" {
		return defaults.Kind
	}

	return defaultKind
}

func (t *Topology) GetNodeGroup(nodeName string) string {
	defaultGroup := t.GetDefaults().Group

	nodeDefinition, ok := t.Nodes[nodeName]
	if nodeDefinition == nil || !ok {
		return defaultGroup
	}

	if nodeDefinition.Group != "" {
		return nodeDefinition.Group
	}

	kind := t.GetNodeKind(nodeName)

	if kind != "" {
		kindGroup := t.GetKind(kind).Group

		if kindGroup != "" {
			return kindGroup
		}
	}

	return defaultGroup
}

func (t *Topology) GetNodeType(nodeName string) string {
	defaultType := t.GetDefaults().Type

	nodeDefinition, ok := t.Nodes[nodeName]
	if nodeDefinition == nil || !ok {
		return defaultType
	}

	if nodeDefinition.Type != "" {
		return strings.TrimSpace(nodeDefinition.Type)
	}

	group := t.GetGroup(t.GetNodeGroup(nodeName))
	if group != nil && group.Type != "" {
		return strings.TrimSpace(group.Type)
	}

	kind := t.GetKind(t.GetNodeKind(nodeName))
	if kind != nil && kind.Type != "" {
		return strings.TrimSpace(kind.Type)
	}

	return defaultType
}

func (t *Topology) GetNodePorts(nodeName string) (nat.PortSet, nat.PortMap, error) {
	ports := getField(
		t,
		nodeName,
		func(node *NodeDefinition) []string { return node.Ports },
		func(group *NodeDefinition) []string { return group.Ports },
		func(kind *NodeDefinition) []string { return kind.Ports },
		func(defaults *NodeDefinition) []string { return defaults.Ports },
		func(v []string) bool { return len(v) > 0 },
	)

	if ports != nil {
		return nat.ParsePortSpecs(ports)
	}

	return nil, nil, nil
}

func (t *Topology) GetNodeEnv(nodeName string) map[string]string {
	return mergeStringMapFields(
		t,
		nodeName,
		func(node *NodeDefinition) map[string]string { return node.Env },
		func(group *NodeDefinition) map[string]string { return group.Env },
		func(kind *NodeDefinition) map[string]string { return kind.Env },
		func(defaults *NodeDefinition) map[string]string { return defaults.Env },
	)
}

func (t *Topology) GetNodeEnvFiles(nodeName string) []string {
	return mergeStringSliceFields(
		t,
		nodeName,
		func(node *NodeDefinition) []string { return node.EnvFiles },
		func(group *NodeDefinition) []string { return group.EnvFiles },
		func(kind *NodeDefinition) []string { return kind.EnvFiles },
		func(defaults *NodeDefinition) []string { return defaults.EnvFiles },
	)
}

func (t *Topology) GetNodeLabels(nodeName string) map[string]string {
	return mergeStringMapFields(
		t,
		nodeName,
		func(node *NodeDefinition) map[string]string { return node.Labels },
		func(group *NodeDefinition) map[string]string { return group.Labels },
		func(kind *NodeDefinition) map[string]string { return kind.Labels },
		func(defaults *NodeDefinition) map[string]string { return defaults.Labels },
	)
}

func (t *Topology) GetNodeDevices(nodeName string) []string {
	return mergeStringSliceFields(
		t,
		nodeName,
		func(node *NodeDefinition) []string { return node.Devices },
		func(group *NodeDefinition) []string { return group.Devices },
		func(kind *NodeDefinition) []string { return kind.Devices },
		func(defaults *NodeDefinition) []string { return defaults.Devices },
	)
}

func (t *Topology) GetNodeCapAdd(nodeName string) []string {
	return mergeStringSliceFields(
		t,
		nodeName,
		func(node *NodeDefinition) []string { return node.CapAdd },
		func(group *NodeDefinition) []string { return group.CapAdd },
		func(kind *NodeDefinition) []string { return kind.CapAdd },
		func(defaults *NodeDefinition) []string { return defaults.CapAdd },
	)
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

func (t *Topology) GetComponents(nodeName string) []*Component {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) []*Component { return node.Components },
		func(group *NodeDefinition) []*Component { return group.Components },
		func(kind *NodeDefinition) []*Component { return kind.Components },
		func(defaults *NodeDefinition) []*Component { return defaults.Components },
		func(v []*Component) bool { return v != nil },
	)
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

func (t *Topology) GetNodeImagePullPolicy(nodeName string) PullPolicyValue {
	return ParsePullPolicyValue(
		getField(
			t,
			nodeName,
			func(node *NodeDefinition) string { return node.ImagePullPolicy },
			func(group *NodeDefinition) string { return group.ImagePullPolicy },
			func(kind *NodeDefinition) string { return kind.ImagePullPolicy },
			func(defaults *NodeDefinition) string { return defaults.ImagePullPolicy },
			func(v string) bool { return v != "" },
		),
	)
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

func (t *Topology) GetNodeExec(nodeName string) []string {
	return mergeStringSliceFields(
		t,
		nodeName,
		func(node *NodeDefinition) []string { return node.Exec },
		func(group *NodeDefinition) []string { return group.Exec },
		func(kind *NodeDefinition) []string { return kind.Exec },
		func(defaults *NodeDefinition) []string { return defaults.Exec },
	)
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
func (t *Topology) GetSysCtl(nodeName string) map[string]string {
	return mergeStringMapFields(
		t,
		nodeName,
		func(node *NodeDefinition) map[string]string { return node.Sysctls },
		func(group *NodeDefinition) map[string]string { return group.Sysctls },
		func(kind *NodeDefinition) map[string]string { return kind.Sysctls },
		func(defaults *NodeDefinition) map[string]string { return defaults.Sysctls },
	)
}

// GetNodeExtras returns the 'extras' section for the given node.
func (t *Topology) GetNodeExtras(nodeName string) *Extras {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) *Extras { return node.Extras },
		func(group *NodeDefinition) *Extras { return group.Extras },
		func(kind *NodeDefinition) *Extras { return kind.Extras },
		func(defaults *NodeDefinition) *Extras { return defaults.Extras },
		func(v *Extras) bool { return v != nil },
	)
}

func (t *Topology) GetNodeDns(nodeName string) *DNSConfig {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) *DNSConfig { return node.DNS },
		func(group *NodeDefinition) *DNSConfig { return group.DNS },
		func(kind *NodeDefinition) *DNSConfig { return kind.DNS },
		func(defaults *NodeDefinition) *DNSConfig {
			if defaults.DNS != nil {
				return defaults.DNS
			}

			return &DNSConfig{}
		},
		func(v *DNSConfig) bool { return v != nil },
	)
}

func (t *Topology) GetHealthCheckConfig(nodeName string) *HealthcheckConfig {
	return getField(
		t,
		nodeName,
		func(node *NodeDefinition) *HealthcheckConfig { return node.HealthCheck },
		func(group *NodeDefinition) *HealthcheckConfig { return group.HealthCheck },
		func(kind *NodeDefinition) *HealthcheckConfig { return kind.HealthCheck },
		func(defaults *NodeDefinition) *HealthcheckConfig { return defaults.HealthCheck },
		func(v *HealthcheckConfig) bool { return v != nil },
	)
}

func (t *Topology) GetNodeBinds(nodeName string) ([]string, error) {
	bindSources := mergeStringSliceFields(
		t,
		nodeName,
		func(node *NodeDefinition) []string { return node.Binds },
		func(group *NodeDefinition) []string { return group.Binds },
		func(kind *NodeDefinition) []string { return kind.Binds },
		func(defaults *NodeDefinition) []string { return defaults.Binds },
	)

	if len(bindSources) == 0 {
		return nil, nil
	}

	binds := map[string]*Bind{}

	// add the binds from less to more specific levels, indexed by the destination path.
	// thereby more specific binds will overwrite less specific one
	for _, bind := range bindSources {
		b, err := NewBindFromString(bind)
		if err != nil {
			return nil, err
		}

		binds[b.Dst()] = b
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

func (t *Topology) GetNodeConfigDispatcher(nodeName string) *ConfigDispatcher {
	nodeDefintion, ok := t.Nodes[nodeName]
	if nodeDefintion == nil || !ok {
		return nil
	}

	vars := t.Defaults.Config.GetVars()

	kind := t.GetKind(t.GetNodeKind(nodeName))
	if kind != nil && kind.Config != nil {
		vars = clabutils.MergeMaps(vars, kind.Config.GetVars())
	}

	group := t.GetGroup(t.GetNodeGroup(nodeName))
	if group != nil && group.Config != nil {
		vars = clabutils.MergeMaps(vars, group.Config.GetVars())
	}

	nodeDefinition := t.Nodes[nodeName]
	if nodeDefinition != nil && nodeDefinition.Config != nil {
		vars = clabutils.MergeMaps(vars, nodeDefinition.Config.GetVars())
	}

	return &ConfigDispatcher{
		Vars: vars,
	}
}

// GetStages return the configuration stages set for the given node.
// It merges the default, kind and node stages into a single Stages struct
// with node stages taking precedence over kind stages and default stages.
func (t *Topology) GetStages(nodeName string) (*Stages, error) {
	s := NewStages()

	defaultStages := t.GetDefaults().Stages
	if defaultStages != nil {
		s.Merge(defaultStages)
	}

	kind := t.GetKind(t.GetNodeKind(nodeName))
	if kind != nil && kind.Stages != nil {
		s.Merge(kind.Stages)
	}

	group := t.GetGroup(t.GetNodeGroup(nodeName))
	if group != nil && group.Stages != nil {
		s.Merge(group.Stages)
	}

	nodeDefinition := t.Nodes[nodeName]
	if nodeDefinition != nil && nodeDefinition.Stages != nil {
		s.Merge(nodeDefinition.Stages)
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

// GetCertificateConfig returns the certificate configuration for the given node.
func (t *Topology) GetCertificateConfig(nodeName string) *CertificateConfig {
	// default for issuing node certificates is false
	cc := &CertificateConfig{
		Issue: clabutils.Pointer(false),
	}

	cc.Merge(t.GetDefaults().Certificate)

	kind := t.GetKind(t.GetNodeKind(nodeName))
	if kind != nil {
		cc.Merge(kind.Certificate)
	}

	group := t.GetGroup(t.GetNodeGroup(nodeName))
	if group != nil {
		cc.Merge(group.Certificate)
	}

	nodeDefinition := t.Nodes[nodeName]
	if nodeDefinition != nil {
		cc.Merge(nodeDefinition.Certificate)
	}

	return cc
}

func (t *Topology) GetNodeAliases(nodeName string) []string {
	nodeDefinition, ok := t.Nodes[nodeName]
	if nodeDefinition == nil || !ok {
		return nil
	}

	return nodeDefinition.Aliases
}
