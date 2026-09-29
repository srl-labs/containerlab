package tailscale

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

const (
	ProxyName = "ts"
	ToolType  = "tailscale"
	ServeFile = "serve.json"
)

func ProxyConfig(labName, longName, network, labDir string) *clabtypes.NodeConfig {
	return &clabtypes.NodeConfig{
		ShortName:     ProxyName,
		LongName:      longName,
		Kind:          "linux",
		NodeType:      "tool",
		Image:         DefaultImage,
		MgmtNet:       network,
		LabDir:        labDir,
		RestartPolicy: "always",
		Binds:         []string{labDir + ":" + tsStateDir},
		Env: map[string]string{
			"TS_USERSPACE":    "true",
			"TS_ACCEPT_DNS":   "false",
			"TS_AUTH_ONCE":    "true",
			"TS_STATE_DIR":    tsStateDir,
			"TS_HOSTNAME":     "clab-" + labName,
			"TS_SERVE_CONFIG": tsStateDir + "/" + ServeFile,
		},
	}
}

type tsServeConfig struct {
	TCP map[string]tsTCPHandler `json:"TCP"`
}

type tsTCPHandler struct {
	TCPForward string `json:"TCPForward"`
}

// WriteServeConfig writes the proxy Serve config for the /ts ports of nodes into labDir.
// The file is replaced atomically because containerboot reloads it on every change and
// exits if it reads a partial file.
func WriteServeConfig(labDir string, nodes map[string]clabnodes.Node) error {
	if labDir == "" {
		return nil
	}

	cfg := tsServeConfig{TCP: map[string]tsTCPHandler{}}
	for _, node := range nodes {
		if node == nil || node.Config() == nil {
			continue
		}
		ip := mgmtIP(node.Config())
		if ip == "" {
			continue
		}
		for _, p := range node.Config().TailscalePorts {
			cfg.TCP[strconv.FormatUint(uint64(p.Listen), 10)] = tsTCPHandler{
				TCPForward: net.JoinHostPort(ip, strconv.FormatUint(uint64(p.Dest), 10)),
			}
		}
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(labDir, "."+ServeFile+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(clabconstants.PermissionsFileDefault); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), filepath.Join(labDir, ServeFile))
}
