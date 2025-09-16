package arrcus_arcos

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"arrcus_arcos"}
	defaultCredentials = clabnodes.NewCredentials("clab", "clab@123")

	// Initialize Env with arcos-specific defaults (currently empty, for future extensibility)
	arcosEnv = map[string]string{}

	//go:embed arcos.cfg
	cfgTemplate string

	ifaceRe = regexp.MustCompile(`^swp[\d.]+$`)
)

type arcos struct {
	clabnodes.DefaultNode
}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, nil)
	r.Register(kindNames, func() clabnodes.Node {
		return new(arcos)
	}, nrea)
}

// Init DefaultNode initialization and apply options.
func (n *arcos) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	for _, o := range opts {
		o(n)
	}

	n.Cfg.Env = clabutils.MergeStringMaps(arcosEnv, n.Cfg.Env)

	// Binds and Env are finalized in PreDeploy
	return nil
}

func (n *arcos) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	// Create LabDir (clabutils.CreateDirectory returns nothing)
	clabutils.CreateDirectory(n.Cfg.LabDir, 0o755)

	// Create/load certificate
	if _, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName); err != nil {
		return err
	}

	// Prepare startup.cfg (copy user-provided if exists, otherwise generate from template)
	if err := n.createARCOSFiles(ctx); err != nil {
		return err
	}

	// Always mount /config/startup.cfg as :ro and set ENV accordingly
	hostCfg := n.Cfg.ResStartupConfig
	containerCfg := "/config/startup.cfg"
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprintf("%s:%s:ro", hostCfg, containerCfg))
	n.Cfg.Env["STARTUP_CFG"] = containerCfg

	return nil
}

func (n *arcos) createARCOSFiles(ctx context.Context) error {
	// Unify destination to $LABDIR/startup.cfg
	dst := filepath.Join(n.Cfg.LabDir, "startup.cfg")
	n.Cfg.ResStartupConfig = dst

	if n.Cfg.StartupConfig != "" {
		// Use user-provided file as-is (no template processing)
		return clabutils.CopyFile(ctx, n.Cfg.StartupConfig, dst, 0o644)
	}

	// If not specified, explicitly expand and write the embedded template
	cfgBuf, err := clabutils.SubstituteEnvsAndTemplate(strings.NewReader(cfgTemplate), n.Cfg)
	if err != nil {
		return err
	}
	return clabutils.CreateFile(dst, cfgBuf.String())
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file is correct.
func (n *arcos) CheckInterfaceName() error {
	for _, e := range n.Endpoints {
		if !ifaceRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("%q interface name %q doesn't match the required pattern. It should be named as swpX (X >= 0), optionally with .subif",
				n.Cfg.ShortName, e.GetIfaceName())
		}
	}
	return nil
}
