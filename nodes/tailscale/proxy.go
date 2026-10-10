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
	return writeFile(labDir, ServeFile, b, clabconstants.PermissionsFileDefault)
}

// writeFile atomically replaces the destination, including existing symlinks, and sets
// permissions on a new file rather than retaining those of an existing auth-key file.
func writeFile(labDir, name string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(labDir, "."+name+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), filepath.Join(labDir, name))
}
