package types

import (
	"github.com/docker/go-connections/nat"
	"github.com/srl-labs/containerlab/utils"
)

// Topology represents a lab topology.
type Topology struct {
	Defaults *NodeDefinition            `yaml:"defaults,omitempty"`
	Kinds    map[string]*NodeDefinition `yaml:"kinds,omitempty"`
	Nodes    map[string]*NodeDefinition `yaml:"nodes,omitempty"`
	Links    []*LinkDefinition          `yaml:"links,omitempty"`
}

func NewTopology() *Topology {
	return &Topology{
		Defaults: new(NodeDefinition),
		Kinds:    make(map[string]*NodeDefinition),
		Nodes:    make(map[string]*NodeDefinition),
		Links:    make([]*LinkDefinition, 0),
	}
}

type LinkConfig struct {
	Endpoints []string
	Labels    map[string]string      `yaml:"labels,omitempty"`
	Vars      map[string]interface{} `yaml:"vars,omitempty"`
	MTU       int                    `yaml:"mtu,omitempty"`
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
	if t == nil {
		return ""
	}
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetKind() != "" {
			return ndef.GetKind()
		}
		return t.GetDefaults().GetKind()
	}
	return ""
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

func (t *Topology) GetNodeStartupConfig(name string) (string, error) {
	var cfg string
	if ndef, ok := t.Nodes[name]; ok {
		cfg = ndef.GetStartupConfig()
		if t.GetKind(t.GetNodeKind(name)).GetStartupConfig() != "" && cfg == "" {
			cfg = t.GetKind(t.GetNodeKind(name)).GetStartupConfig()
		}
		if cfg == "" {
			cfg = t.GetDefaults().GetStartupConfig()
		}
	}
	return cfg, nil
}

func (t *Topology) GetNodeStartupDelay(name string) uint {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetStartupDelay() != 0 {
			return ndef.GetStartupDelay()
		}
		if t.GetKind(t.GetNodeKind(name)).GetStartupDelay() != 0 {
			return t.GetKind(t.GetNodeKind(name)).GetStartupDelay()
		}
		return t.GetDefaults().GetStartupDelay()
	}
	return 0
}

func (t *Topology) GetNodeEnforceStartupConfig(name string) bool {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetEnforceStartupConfig() {
			return true
		}
		if t.GetKind(t.GetNodeKind(name)).GetEnforceStartupConfig() {
			return true
		}
		return t.GetDefaults().GetEnforceStartupConfig()
	}
	return false
}

func (t *Topology) GetNodeAutoRemove(name string) *bool {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetAutoRemove() != nil {
			return ndef.AutoRemove
		}
		if t.GetKind(t.GetNodeKind(name)).GetAutoRemove() != nil {
			return t.GetKind(t.GetNodeKind(name)).GetAutoRemove()
		}
	}

	if t.GetDefaults().GetAutoRemove() != nil {
		return t.GetDefaults().GetAutoRemove()
	}

	def := false
	return &def
}

func (t *Topology) GetNodeLicense(name string) (string, error) {
	var license string
	if ndef, ok := t.Nodes[name]; ok {

		license = ndef.GetLicense()
		if t.GetKind(t.GetNodeKind(name)).GetLicense() != "" && license == "" {
			license = t.GetKind(t.GetNodeKind(name)).GetLicense()
		}

		if license == "" {
			license = t.GetDefaults().GetLicense()
		}

	}

	return license, nil
}

func (t *Topology) GetNodeImage(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetImage() != "" {
			return ndef.GetImage()
		}
		if t.GetKind(t.GetNodeKind(name)).GetImage() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetImage()
		}
		return t.GetDefaults().GetImage()
	}
	return ""
}

func (t *Topology) GetNodeImagePullPolicy(name string) PullPolicyValue {
	if ndef, ok := t.Nodes[name]; ok {
		if pp := ndef.GetImagePullPolicy(); pp != "" {
			return ParsePullPolicyValue(pp)
		}
		if pp := t.GetKind(t.GetNodeKind(name)).GetImagePullPolicy(); pp != "" {
			return ParsePullPolicyValue(pp)
		}
		if pp := t.GetDefaults().GetImagePullPolicy(); pp != "" {
			return ParsePullPolicyValue(pp)
		}
	}
	// let ParsePullPolicyValue return a default value for missing pull policy.
	return ParsePullPolicyValue("")
}

func (t *Topology) GetNodeGroup(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetGroup() != "" {
			return ndef.GetGroup()
		}
		if t.GetKind(t.GetNodeKind(name)).GetGroup() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetGroup()
		}
		return t.GetDefaults().GetGroup()
	}
	return ""
}

func (t *Topology) GetNodeType(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetType() != "" {
			return ndef.GetType()
		}
		if t.GetKind(t.GetNodeKind(name)).GetType() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetType()
		}
		return t.GetDefaults().GetType()
	}
	return ""
}

func (t *Topology) GetNodePosition(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetPostion() != "" {
			return ndef.GetPostion()
		}
		if t.GetKind(t.GetNodeKind(name)).GetPostion() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetPostion()
		}
		return t.GetDefaults().GetPostion()
	}
	return ""
}

func (t *Topology) GetNodeEntrypoint(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetEntrypoint() != "" {
			return ndef.GetEntrypoint()
		}
		if t.GetKind(t.GetNodeKind(name)).GetEntrypoint() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetEntrypoint()
		}
		return t.GetDefaults().GetEntrypoint()
	}
	return ""
}

func (t *Topology) GetNodeCmd(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetCmd() != "" {
			return ndef.GetCmd()
		}
		if t.GetKind(t.GetNodeKind(name)).GetCmd() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetCmd()
		}
		return t.GetDefaults().GetCmd()
	}
	return ""
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
		if ndef.GetUser() != "" {
			return ndef.GetUser()
		}
		if t.GetKind(t.GetNodeKind(name)).GetUser() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetUser()
		}
		return t.GetDefaults().GetUser()
	}
	return ""
}

func (t *Topology) GetNodeNetworkMode(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNetworkMode() != "" {
			return ndef.GetNetworkMode()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNetworkMode() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNetworkMode()
		}
		return t.GetDefaults().GetNetworkMode()
	}
	return ""
}

func (t *Topology) GetNodeSandbox(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeSandbox() != "" {
			return ndef.GetNodeSandbox()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeSandbox() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeSandbox()
		}
		return t.GetDefaults().GetNodeSandbox()
	}
	return ""
}

func (t *Topology) GetNodeKernel(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeKernel() != "" {
			return ndef.GetNodeKernel()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeKernel() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeKernel()
		}
		return t.GetDefaults().GetNodeKernel()
	}
	return ""
}

func (t *Topology) GetNodeRuntime(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeRuntime() != "" {
			return ndef.GetNodeRuntime()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeRuntime() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeRuntime()
		}
		return t.GetDefaults().GetNodeRuntime()
	}
	return ""
}

func (t *Topology) GetNodeCPU(name string) float64 {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeCPU() != 0 {
			return ndef.GetNodeCPU()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeCPU() != 0 {
			return t.GetKind(t.GetNodeKind(name)).GetNodeCPU()
		}
		return t.GetDefaults().GetNodeCPU()
	}
	return 0
}

func (t *Topology) GetNodeCPUSet(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeCPUSet() != "" {
			return ndef.GetNodeCPUSet()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeCPUSet() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeCPUSet()
		}
		return t.GetDefaults().GetNodeCPUSet()
	}
	return ""
}

func (t *Topology) GetNodeMemory(name string) string {
	if ndef, ok := t.Nodes[name]; ok {
		if ndef.GetNodeMemory() != "" {
			return ndef.GetNodeMemory()
		}
		if t.GetKind(t.GetNodeKind(name)).GetNodeMemory() != "" {
			return t.GetKind(t.GetNodeKind(name)).GetNodeMemory()
		}
		return t.GetDefaults().GetNodeMemory()
	}
	return ""
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

// GetSANs return the Subject Alternative Name configuration for the given node.
func (t *Topology) GetSANs(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		if len(ndef.GetSANs()) > 0 {
			return ndef.GetSANs()
		}
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

		return t.GetDefaults().GetExtras()
	}
	return nil
}

// GetWaitFor return the wait-for configuration for the given node.
func (t *Topology) GetWaitFor(name string) []string {
	if ndef, ok := t.Nodes[name]; ok {
		return utils.MergeStringSlices(
			t.GetKind(t.GetNodeKind(name)).GetWaitFor(),
			ndef.GetWaitFor())
	}
	return nil
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

		return t.GetDefaults().GetDns()
	}
	return nil
}

func (t *Topology) GetCertificateConfig(name string) *CertificateConfig {
	if ndef, ok := t.Nodes[name]; ok {
		nodeCertConf := ndef.GetCertificateConfig()
		if nodeCertConf != nil {
			return nodeCertConf
		}

		kindCertConf := t.GetKind(t.GetNodeKind(name)).GetCertificateConfig()
		if kindCertConf != nil {
			return kindCertConf
		}

		return t.GetDefaults().GetCertificateConfig()
	}

	return nil
}
