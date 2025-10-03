package cvce

/*

Better startup experience:
- Bootstrapping config
  - Schema
    - Interface IP configs
	- Edge ID
  - Mount config into container
    - /clab-data/config
- VCO automation
  - Check if edge activation pending via VCO API
    - If not, call reactivate API
	- Finally, get activation key
  - Use VCO API to update edge interface IPs to match user's config
    - Phase 1 of this should be simply validating that configs match between YAML & VCO
	- Later on, rely on VeloAVD to do more advanced provisioning
	  - AVD configuration would be used to derive the initial cvce container configs
	  - AVD would handle configuring all VCO elements
  - Mount activation key + VCO FQDN as file in container
    - /clab-data/activation-info
- Startup script
  - Wait for things to settle
  - Parse config
  - Parse activation key
  - Run set_wan_config.sh for all interfaces
  - Run activate.py -f -s ... <key>

Better post-deploy experience:
- Make SSH work via VCO automation

VCE node config
- Add vco-fqdn, enterprise-id, api-token, edge-id to node spec
- Use groups to define common Velo enterprise settings
- API token should generally come from an env var
- Example below

topology:
  groups:
    velo-ent1:
      kind: cvce
  	  vco-fqdn: veco58-kiad1.velocloud.net
  	  enterprise-id: 1727
  	  api-token: xxxx
  nodes:
    branch1:
	  group: velo-ent1
	  edge-id: 1234

*/

/*
name: 3m-topo

topology:
  kinds:
    arista_ceos:
      binds:
        - ceos-intfmap.json:/mnt/flash/EosIntfMapping.json:ro
  groups:
    homelab-ent:
      kind: cvce
      image: edge:r6110-163149
      vco-fqdn: veco58-kiad1.velocloud.net
      enterprise-id: 1727
      api-token: ${VCO_API_TOKEN}
  nodes:
    ceos-inet:
      kind: ceos
      image: ceos:4.34.2.1F
    ceos-ch1-1:
      kind: ceos
      image: ceos:4.34.2.1F
    ceos-ch1-2:
      kind: ceos
      image: ceos:4.34.2.1F
    ceos-ty6-1:
      kind: ceos
      image: ceos:4.34.2.1F
    ceos-ty6-2:
      kind: ceos
      image: ceos:4.34.2.1F
    ceos-gb:
      kind: ceos
      image: ceos:4.34.2.1F
    ch1-server:
      kind: linux
      image: alpine:latest
    b-client:
      kind: linux
      image: alpine:latest
    b-1:
      kind: cvce
      image: edge:r6110-163149
      # VCE doesn't tolerate docker's resolv.conf behavior. i have to bind-mount a custom resolv.conf to override it or edge DNS fails.
      # this is done with some patches to containerlab.
      # --no-dns
      cpu: 2 # minimum 2
      cpu-set: 0,3 # Linux can schedule many containers fine but edged tries to pin threads which conflicts.
      # add code in containerlab to probe host CPU
      memory: 4 GB # edge takes all RAM & CPU available during startup which fails.
      # maybe 2 GB RAM
      # disable huge-pages
      # resource probing needs some kind of hard-override to prevent edged from using the entire host.
      cap-add: # ideally no need for cap-add ALL. not sure why its necessary.
        - ALL
        # CAP_NET_ADMIN
        # setaffinity privs - CAP_SYS_NICE
        # /dev/net/tun - parent tun intf
      network-mode: none
      # ideally a dedicated Mgmt network as the first ethernet interface. not for DP, only to allow mgmt access to VCE.
      # containers generally expect to have a mgmt interface and then other interfaces after that.
      # containerlab uses that management network to populate ansible inventories and automate SSH connectivity, startup commands, etc.
    gb:
      kind: cvce
      image: edge:r6110-163149
      cpu: 2
      cpu-set: 0,3
      memory: 4 GB
      cap-add:
        - ALL
      network-mode: none
    ch1-1:
      kind: cvce
      image: edge:r6110-163149
      cpu: 2
      cpu-set: 0,3
      memory: 4 GB
      cap-add:
        - ALL
      network-mode: none
    ch1-2:
      kind: cvce
      image: edge:r6110-163149
      cpu: 2
      cpu-set: 0,3
      memory: 4 GB
      cap-add:
        - ALL
      network-mode: none
    ty6-1:
      kind: cvce
      image: edge:r6110-163149
      cpu: 2
      cpu-set: 0,3
      memory: 4 GB
      cap-add:
        - ALL
      network-mode: none
    ty6-2:
      kind: cvce
      image: edge:r6110-163149
      cpu: 2
      cpu-set: 0,3
      memory: 4 GB
      cap-add:
        - ALL
      network-mode: none
  links:
    - endpoints: ["b-1:eth0", "b-client:eth1"]
    - endpoints: ["b-1:eth2", "ceos-inet:eth1"]
    - endpoints: ["gb:eth2", "ceos-inet:eth2"]
    - endpoints: ["gb:eth0", "ceos-gb:eth1"]
    - endpoints: ["ch1-1:eth2", "ceos-inet:eth3"]
    - endpoints: ["ch1-2:eth2", "ceos-inet:eth4"]
    - endpoints: ["ty6-1:eth2", "ceos-inet:eth5"]
    - endpoints: ["ty6-2:eth2", "ceos-inet:eth6"]
    - endpoints: ["ty6-1:eth0", "ceos-ty6-1:eth1"]
    - endpoints: ["ty6-1:eth1", "ceos-ty6-2:eth1"]
    - endpoints: ["ty6-2:eth0", "ceos-ty6-1:eth2"]
    - endpoints: ["ty6-2:eth1", "ceos-ty6-2:eth2"]
    - endpoints: ["ceos-ty6-1:eth3", "ceos-ty6-2:eth3"]
    - endpoints: ["ch1-1:eth0", "ceos-ch1-1:eth1"]
    - endpoints: ["ch1-1:eth1", "ceos-ch1-2:eth1"]
    - endpoints: ["ch1-2:eth0", "ceos-ch1-1:eth2"]
    - endpoints: ["ch1-2:eth1", "ceos-ch1-2:eth2"]
    - endpoints: ["ceos-ch1-1:eth3", "ceos-ch1-2:eth3"]
    - endpoints: ["ceos-ch1-1:eth4", "ch1-server:eth1"]
    - endpoints: ["ceos-ch1-2:eth4", "ch1-server:eth2"]
    - endpoints: ["ceos-inet:eth7", "macvlan:eth999.200"]

*/

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

var (
	KindNames = []string{"cvce", "arista_cvce"}

	defaultCredentials = clabnodes.NewCredentials("root", "password")
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(KindNames, func() clabnodes.Node {
		return new(cvce)
	}, nrea)
}

type cvce struct {
	clabnodes.DefaultNode

	resolvConfPath    string
	configPath        string
	activationPath    string
	startupScriptPath string

	vcoFqdn       string
	apiToken      string
	enterpriseId  int
	edgeId        int
	activationKey string

	topologyName string
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
}

func (n *cvce) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

	n.HostRequirements.MinVCPU = 2
	n.HostRequirements.MinVCPUFailAction = clabtypes.FailBehaviourError

	n.HostRequirements.MinAvailMemoryGb = 2
	n.HostRequirements.MinAvailMemoryGbFailAction = clabtypes.FailBehaviourError

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	hwa, err := clabutils.GenMac("f0:8e:db")
	if err != nil {
		return err
	}
	n.Cfg.MacAddress = hwa.String()

	// Create interfaces, set process affinity
	// TODO: Make sure edge can init properly with these. Add other if necessary.
	// TODO: Don't blindly overwrite user's settings.
	n.Cfg.CapAdd = []string{
		"CAP_NET_ADMIN",
		"CAP_SYS_NICE",
	}

	// TODO: Don't blindly overwrite user's settings.
	n.Cfg.CPU = 2
	n.Cfg.CPUSet = "0,3"

	// TODO: Make sure it can work OK with 2GB memory. May need to go to 4GB.
	// TODO: Don't blindly overwrite user's settings.
	n.Cfg.Memory = "2048MB"

	// TODO: Revert this once Velo supports dedicated management interfaces
	n.Cfg.NetworkMode = "none"

	// Explicitly bind-mount /etc/resolv.conf to avoid docker's shenanigans
	// TODO: find better workaround for this
	n.resolvConfPath = filepath.Join(n.Cfg.LabDir, "resolv.conf")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.resolvConfPath, ":/etc/resolv.conf"))

	// Mount configuration file
	n.configPath = filepath.Join(n.Cfg.LabDir, "edge-config")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.configPath, ":/clab-data/edge-config"))

	// Mount activation file
	n.activationPath = filepath.Join(n.Cfg.LabDir, "activation-info")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.activationPath, ":/clab-data/activation-info"))

	// Mount setup script
	n.startupScriptPath = filepath.Join(n.Cfg.LabDir, "startup-script")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.startupScriptPath, ":/clab-data/startup-script"))

	n.vcoFqdn = n.Cfg.VeloVcoFqdn
	n.apiToken = n.Cfg.VeloApiToken
	n.enterpriseId = n.Cfg.VeloEnterpriseId
	n.edgeId = n.Cfg.VeloEdgeId

	return nil
}

func (n *cvce) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)

	clabutils.CreateFile(n.resolvConfPath, ResolvConfText)

	if !clabutils.FileExists(n.configPath) {
		clabutils.CreateFile(n.configPath, "")
	}

	err := n.GetActivationKey()
	if err != nil {
		return err
	}

	// Later on, other API calls:
	// - edge/getEdgeConfigurationStack [not necessary for MVP]
	// - configuration/updateConfigurationModule [not necessary for MVP]

	// create activation-info file
	activationText := fmt.Sprint("key = ", n.activationKey, "\nvco_fqdn = ", n.vcoFqdn)
	clabutils.CreateFile(n.activationPath, activationText)

	// Write setup script into mount
	clabutils.CreateFile(n.startupScriptPath, StartupScriptText)

	n.topologyName = params.TopologyName
	n.sshPubKeys = params.SSHPubKeys

	return nil
}

func (n *cvce) PostDeploy(ctx context.Context, params *clabnodes.PostDeployParams) error {
	cmd := exec.ExecCmd{
		Cmd: []string{
			"/usr/bin/python",
			"/clab-data/startup-script",
		},
	}
	_, err := n.RunExec(ctx, &cmd)

	return err
}

type JsonRpcRequest[T interface{}] struct {
	JsonRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  T      `json:"params"`
}

type JsonRpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func (e *JsonRpcError) Error() string {
	return e.Message
}

type JsonRpcResponse[T interface{}] struct {
	JsonRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Result  *T            `json:"result"`
	Error   *JsonRpcError `json:"error"`
}

func DoPortal[T interface{}, U interface{}](vcoUrl string, tokenValue string, method string, params T) (*U, error) {
	request := JsonRpcRequest[T]{
		JsonRPC: "2.0",
		ID:      "1",
		Method:  method,
		Params:  params,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", vcoUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", tokenValue)
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonRpcResponse JsonRpcResponse[U]
	if err := json.NewDecoder(resp.Body).Decode(&jsonRpcResponse); err != nil {
		return nil, err
	}

	if jsonRpcResponse.Result != nil {
		return jsonRpcResponse.Result, nil
	} else {
		return nil, jsonRpcResponse.Error
	}
}

type GetEdgeRequest struct {
	EnterpriseId *int `json:"enterpriseId,omitempty"`
	EdgeId       int  `json:"edgeId"`
}
type GetEdgeResponse struct {
	ActivationKey   string `json:"activationKey"`
	ActivationState string `json:"activationState"`
	EdgeState       string `json:"edgeState"`
}

type RequestReactivationRequest struct {
	EnterpriseId *int `json:"enterpriseId,omitempty"`
	EdgeId       int  `json:"edgeId"`
}

type RequestReactivationResponse struct {
	ActivationKey        string `json:"activationKey"`
	ActivationKeyExpires string `json:"activationKeyExpires"`
}

func (n *cvce) GetActivationKey() error {
	// - edge/getEdge - check if activated, get RMA code
	edge, err := n.GetEdge()
	if err != nil {
		return err
	}

	if strings.Contains(edge.ActivationState, "REACTIVATION_PENDING") {
		n.activationKey = edge.ActivationKey
		return nil
	}

	if strings.Contains(edge.EdgeState, "CONNECTED") || strings.Contains(edge.EdgeState, "DEGRADED") {
		log.Warn(fmt.Sprintf("edge state is %s", edge.EdgeState))
	}

	// - edge/requestReactivation - request new RMA code
	reactivationInfo, err := n.RequestReactivation()
	if err != nil {
		return err
	}

	n.activationKey = reactivationInfo.ActivationKey

	return nil
}

func (n *cvce) GetEdge() (*GetEdgeResponse, error) {
	vcoUrl := fmt.Sprintf("https://%s/portal/", n.vcoFqdn)
	tokenValue := fmt.Sprintf("Token %s", n.apiToken)

	// - edge/getEdge - check if activated, get RMA code
	getEdgeRequest := GetEdgeRequest{
		EnterpriseId: nil,
		EdgeId:       n.edgeId,
	}
	if n.enterpriseId != 0 {
		getEdgeRequest.EnterpriseId = &n.enterpriseId
	}

	resp, err := DoPortal[GetEdgeRequest, GetEdgeResponse](vcoUrl, tokenValue, "edge/getEdge", getEdgeRequest)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (n *cvce) RequestReactivation() (*RequestReactivationResponse, error) {
	vcoUrl := fmt.Sprintf("https://%s/portal/", n.vcoFqdn)
	tokenValue := fmt.Sprintf("Token %s", n.apiToken)

	// - edge/getEdge - check if activated, get RMA code
	req := RequestReactivationRequest{
		EnterpriseId: nil,
		EdgeId:       n.edgeId,
	}
	if n.enterpriseId != 0 {
		req.EnterpriseId = &n.enterpriseId
	}

	resp, err := DoPortal[RequestReactivationRequest, RequestReactivationResponse](vcoUrl, tokenValue, "edge/edgeRequestReactivation", req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *cvce) CheckInterfaceName() error {
	foundGe3 := false
	ifRe := regexp.MustCompile(`eth[0-7]$`)
	for _, e := range n.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("cvce node %q has an interface named %q which doesn't match the required pattern. Interfaces should be named eth0-eth7, where eth0 -> GE1, and so on", n.Cfg.ShortName, e.GetIfaceName())
		}

		foundGe3 = foundGe3 || e.GetIfaceName() == "eth2"
	}

	if !foundGe3 {
		return fmt.Errorf("cvce node %q must have an interface named eth2 (mapped to GE3) for activation", n.Cfg.ShortName)
	}

	return nil
}
