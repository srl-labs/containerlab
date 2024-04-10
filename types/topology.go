package types

import (
	"github.com/docker/go-connections/nat"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/utils"
)

// Topology represents a lab topology.
type Topology struct {
	Defaults *NodeDefinition            `yaml:"defaults,omitempty"`
	Kinds    map[string]*NodeDefinition `yaml:"kinds,omitempty"`
	Nodes    map[string]*NodeDefinition `yaml:"nodes,omitempty"`
	Links    []*links.LinkDefinition    `yaml:"links,omitempty"`
}

func NewTopology() *Topology {
	return &Topology{
		Defaults: new(NodeDefinition),
		Kinds:    make(map[string]*NodeDefinition),
		Nodes:    make(map[string]*NodeDefinition),
		Links:    make([]*links.LinkDefinition, 0),
	}
}

func (t *Topology) GetDefaults() *NodeDefinition {
	if t.Defaults != nil {
		return t.Defaults
	}
	return new(NodeDefinition)
}

func (t *Topology) GetKind(kind string) *NodeDefinition {
	if t.Kinds == nil {
		return new(NodeDefinition)
	}
	if kdef, ok := t.Kinds[kind]; ok {
		return kdef
	}
	return new(NodeDefinition)
}

func (t *Topology) GetKinds() map[string]*NodeDefinition {
	if t.Kinds == nil {
		return make(map[string]*NodeDefinition)
	}
	return t.Kinds
}

func (t *Topology) GetNodeKind(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetKind(); v != "" {
			return v
		}
		// if no node kind is set, there is no way for us to look up the kind in the kind ... obviousely
	}
	return t.GetDefaults().GetKind()
}

func (t *Topology) GetNodeBinds(name string) ([]string, error) {
	if _, ok := t.Nodes[name]; !ok {
		return nil, nil
	}

	binds := map[string]*Bind{}

	// group the default, kind and node binds
	bindSources := [][]string{
		t.GetDefaults().GetBinds(),
		t.GetKind(t.GetNodeKind(name)).GetBinds(), t.Nodes[name].GetBinds(),
	}

	// add the binds from less to more specific levels, indexed by the destination path.
	// thereby more specific binds will overwrite less specific one
	for _, bs := range bindSources {
		for _, bind := range bs {
			b, err := NewBind(bind)
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
		return utils.MergeStringMaps(
			utils.MergeStringMaps(t.GetDefaults().GetEnv(),
				t.GetKind(t.GetNodeKind(name)).GetEnv()),
			ndef.GetEnv())
	}
	return nil
}

func (t *Topology) GetNodeEnvFiles(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		return utils.MergeStringSlices(
			utils.MergeStringSlices(t.GetDefaults().GetEnvFiles(),
				t.GetKind(t.GetNodeKind(name)).GetEnvFiles()),
			ndef.GetEnvFiles())
	}
	return nil
}

func (t *Topology) GetNodePublish(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		if len(ndef.GetPublish()) > 0 {
			return ndef.GetPublish()
		}
		if kdef, ok := t.Kinds[ndef.GetKind()]; ok && kdef != nil {
			if len(kdef.GetPublish()) > 0 {
				return kdef.GetPublish()
			}
		}
		return t.Defaults.GetPublish()
	}
	return nil
}

func (t *Topology) GetNodeLabels(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return utils.MergeStringMaps(t.Defaults.GetLabels(),
			t.GetKind(t.GetNodeKind(name)).GetLabels(),
			ndef.GetLabels())
	}
	return nil
}

func (t *Topology) GetNodeConfigDispatcher(name string) *ConfigDispatcher {
	if ndef, ok := t.Nodes[name]; ok {
		vars := utils.MergeMaps(t.Defaults.GetConfigDispatcher().GetVars(),
			t.GetKind(t.GetNodeKind(name)).GetConfigDispatcher().GetVars(),
			ndef.GetConfigDispatcher().GetVars())

		return &ConfigDispatcher{
			Vars: vars,
		}
	}

	return nil
}

func (t *Topology) GetNodeStartupConfig(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetStartupConfig(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetStartupConfig(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetStartupConfig()
}

func (t *Topology) GetNodeStartupDelay(name string) uint {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetStartupDelay(); v != 0 {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetStartupDelay(); v != 0 {
			return v
		}
	}
	return t.GetDefaults().GetStartupDelay()
}

func (t *Topology) GetNodeEnforceStartupConfig(name string) bool {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetEnforceStartupConfig(); v != nil {
			return *v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetEnforceStartupConfig(); v != nil {
			return *v
		}
	}
	if v := t.GetDefaults().GetEnforceStartupConfig(); v != nil {
		return *v
	}
	return false
}

func (t *Topology) GetNodeSuppressStartupConfig(name string) bool {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetSuppressStartupConfig() != nil {
			return *ndef.SuppressStartupConfig
		}
		if r := t.GetKind(t.GetNodeKind(name)).GetSuppressStartupConfig(); r != nil {
			return *r
		}
		if r := t.GetDefaults().GetSuppressStartupConfig(); r != nil {
			return *r
		}
	}
	return false
}

func (t *Topology) GetNodeAutoRemove(name string) bool {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetAutoRemove(); v != nil {
			return *v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetAutoRemove(); v != nil {
			return *v
		}
	}
	if v := t.GetDefaults().GetAutoRemove(); v != nil {
		return *v
	}
	return false
}

func (t *Topology) GetNodeLicense(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if l := ndef.GetLicense(); l != "" {
			return l
		}
		if l := t.GetKind(t.GetNodeKind(name)).GetLicense(); l != "" {
			return l
		}
	}
	return t.GetDefaults().GetLicense()
}

func (t *Topology) GetNodeImage(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetImage(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetImage(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetImage()
}

func (t *Topology) GetNodeImagePullPolicy(name string) PullPolicyValue {
	if ndef, ok := t.Nodes[name]; ok {
		if pp := ndef.GetImagePullPolicy(); pp != "" {
			return ParsePullPolicyValue(pp)
		}
		if pp := t.GetKind(t.GetNodeKind(name)).GetImagePullPolicy(); pp != "" {
			return ParsePullPolicyValue(pp)
		}
	}
	return ParsePullPolicyValue(t.GetDefaults().GetImagePullPolicy())
}

func (t *Topology) GetNodeGroup(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetGroup(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetGroup(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetGroup()
}

func (t *Topology) GetNodeType(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetType(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetType(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetType()
}

func (t *Topology) GetNodePosition(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetPostion(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetPostion(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetPostion()
}

func (t *Topology) GetNodeEntrypoint(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetEntrypoint(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetEntrypoint(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetEntrypoint()
}

func (t *Topology) GetNodeCmd(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetCmd(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetCmd(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetCmd()
}

func (t *Topology) GetNodeExec(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		d := t.GetDefaults().GetExec()
		k := t.GetKind(t.GetNodeKind(name)).GetExec()
		n := ndef.GetExec()

		return append(append(d, k...), n...)
	}
	return nil
}

func (t *Topology) GetNodeUser(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetUser(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetUser(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetUser()
}

func (t *Topology) GetNodeNetworkMode(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetNetworkMode(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetNetworkMode(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetNetworkMode()
}

func (t *Topology) GetNodeSandbox(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetNodeSandbox(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetNodeSandbox(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetNodeSandbox()
}

func (t *Topology) GetNodeKernel(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetNodeKernel(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetNodeKernel(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetNodeKernel()
}

func (t *Topology) GetNodeRuntime(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetNodeRuntime(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetNodeRuntime(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetNodeRuntime()
}

func (t *Topology) GetNodeCPU(name string) float64 {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetNodeCPU(); v != 0 {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetNodeCPU(); v != 0 {
			return v
		}
	}
	return t.GetDefaults().GetNodeCPU()
}

func (t *Topology) GetNodeCPUSet(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetNodeCPUSet(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetNodeCPUSet(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetNodeCPUSet()
}

func (t *Topology) GetNodeMemory(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if v := ndef.GetNodeMemory(); v != "" {
			return v
		}
		if v := t.GetKind(t.GetNodeKind(name)).GetNodeMemory(); v != "" {
			return v
		}
	}
	return t.GetDefaults().GetNodeMemory()
}

// GetSysCtl return the Sysctl configuration for the given node.
func (t *Topology) GetSysCtl(name string) map[string]string {
	if ndef, ok := t.Nodes[name]; ok {
		return utils.MergeStringMaps(
			utils.MergeStringMaps(t.GetDefaults().GetSysctls(),
				t.GetKind(t.GetNodeKind(name)).GetSysctls()),
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

	// node Stages
	nodeStages := t.Nodes[name].GetStages()
	if nodeStages != nil {
		s.Merge(nodeStages)
	}

	return s, nil
}

func (t *Topology) ImportEnvs() {
	t.Defaults.ImportEnvs()

	for _, k := range t.Kinds {
		k.ImportEnvs()
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
		Issue: utils.Pointer(false),
	}

	// merge defaults, kind and node certificate config into the default certificate config
	cc.Merge(
		t.GetDefaults().GetCertificateConfig()).Merge(
		t.GetKind(t.GetNodeKind(name)).GetCertificateConfig()).Merge(
		t.Nodes[name].GetCertificateConfig())

	return cc
}

func (t *Topology) GetHealthCheckConfig(name string) *HealthcheckConfig {
	if ndef, ok := t.Nodes[name]; ok {
		nodeHealthcheckConf := ndef.GetHealthcheckConfig()
		if nodeHealthcheckConf != nil {
			return nodeHealthcheckConf
		}

		kindHealthcheckConf := t.GetKind(t.GetNodeKind(name)).GetHealthcheckConfig()
		if kindHealthcheckConf != nil {
			return kindHealthcheckConf
		}

		return t.GetDefaults().GetHealthcheckConfig()
	}

	return nil
}
