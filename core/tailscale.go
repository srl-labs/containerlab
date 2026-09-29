package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodestailscale "github.com/srl-labs/containerlab/nodes/tailscale"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func isInternalNode(n clabnodes.Node) bool {
	if n == nil || n.Config() == nil {
		return false
	}

	return n.Config().Labels[clabconstants.InternalNode] == "true"
}

func tailscaleSidecarEligible(cfg *clabtypes.NodeConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.Kind == clabnodestailscale.KindName {
		return false
	}
	if cfg.Labels[clabconstants.InternalNode] == "true" {
		return false
	}
	if len(cfg.Components) > 0 {
		return false
	}

	return cfg.ManagementIPAMEligible()
}

func (c *CLab) injectTailscaleSidecars() error {
	if c.Config == nil || c.Config.Mgmt == nil || c.Config.Mgmt.Tailscale == nil {
		return nil
	}

	if c.Config.Mgmt.Tailscale.Proxy() || strings.TrimSpace(c.Config.Mgmt.Tailscale.AuthKey) == "" {
		return nil
	}

	parents := make([]string, 0, len(c.Nodes))
	for name, n := range c.Nodes {
		if len(n.Config().TailscalePorts) > 0 {
			log.Warn("Tailscale /ts ports are only used with auth-mode: sso, ignoring", "node", name)
		}
		if tailscaleSidecarEligible(n.Config()) {
			parents = append(parents, name)
		}
	}
	sort.Strings(parents)

	for _, parent := range parents {
		if err := c.addTailscaleNode(
			clabnodestailscale.SidecarName(parent),
			c.nodeRuntime(parent),
			map[string]string{
				clabconstants.InternalNode: "true",
				clabconstants.RootNodeName: parent,
			},
			[]string{parent},
		); err != nil {
			return err
		}
	}

	return nil
}

func (c *CLab) tailscaleProxyEnabled() bool {
	return c.Config.Mgmt != nil && c.Config.Mgmt.Tailscale.Proxy()
}

// verifyTailscaleProxy checks that the SSO proxy container name is free and that every
// /ts listen port is unique on the proxy.
func (c *CLab) verifyTailscaleProxy() error {
	if !c.tailscaleProxyEnabled() {
		return nil
	}
	if _, exists := c.Nodes[clabnodestailscale.ProxyName]; exists {
		return fmt.Errorf(
			"node name %q is reserved for the Tailscale SSO proxy",
			clabnodestailscale.ProxyName,
		)
	}

	owners := map[uint16]string{}
	for _, name := range sortedNodeNames(c.Nodes) {
		cfg := c.Nodes[name].Config()
		if len(cfg.TailscalePorts) > 0 && !cfg.ManagementIPAMEligible() {
			log.Warn("Tailscale /ts ports need a node management IP, ignoring", "node", name)
			continue
		}
		for _, p := range cfg.TailscalePorts {
			if owner, exists := owners[p.Listen]; exists {
				return fmt.Errorf(
					"Tailscale listen port %d is used by nodes %q and %q",
					p.Listen, owner, name,
				)
			}
			owners[p.Listen] = name
		}
	}

	return nil
}

// syncTailscaleProxy reconciles the SSO proxy tool container with mgmt.tailscale. It removes
// the proxy when SSO is off, using the Serve config file as a marker that a proxy was
// created so labs without one skip the runtime lookup; otherwise it rewrites the Serve config from the nodes' current
// /ts ports and management IPs, which a running proxy reloads without a restart, and creates
// the proxy when it is missing. Node runtime info must be current.
func (c *CLab) syncTailscaleProxy(ctx context.Context) error {
	labDir := c.TopoPaths.NodeDir(clabnodestailscale.ProxyName)
	serveFile := filepath.Join(labDir, clabnodestailscale.ServeFile)
	if !c.tailscaleProxyEnabled() && !clabutils.FileExists(serveFile) {
		return nil
	}

	existing, err := c.ListContainers(
		ctx,
		WithListLabName(c.Config.Name),
		WithListToolType(clabnodestailscale.ToolType),
	)
	if err != nil {
		return fmt.Errorf("listing Tailscale proxy: %w", err)
	}

	if !c.tailscaleProxyEnabled() {
		for idx := range existing {
			name := strings.TrimPrefix(existing[idx].Names[0], "/")
			log.Info("Removing Tailscale proxy", "container", name)
			if err := c.globalRuntime().DeleteContainer(ctx, name); err != nil {
				return fmt.Errorf("removing Tailscale proxy %q: %w", name, err)
			}
		}
		return os.Remove(serveFile)
	}
	if c.skipMgmtNetwork() {
		log.Warn("Tailscale SSO proxy needs the management network, skipping")
		return nil
	}

	clabutils.CreateDirectory(labDir, clabconstants.PermissionsOpen)
	if err := clabnodestailscale.WriteServeConfig(labDir, c.Nodes); err != nil {
		return fmt.Errorf("writing Tailscale Serve config: %w", err)
	}
	if len(existing) > 0 {
		return nil
	}

	return c.createTailscaleProxy(ctx, labDir)
}

func (c *CLab) createTailscaleProxy(ctx context.Context, labDir string) error {
	rt := c.globalRuntime()
	cfg := clabnodestailscale.ProxyConfig(
		c.Config.Name,
		c.nodeLongName(clabnodestailscale.ProxyName),
		c.Config.Mgmt.Network,
		labDir,
	)
	c.addDefaultLabels(cfg)
	cfg.Labels[clabconstants.ToolType] = clabnodestailscale.ToolType
	cfg.Labels[clabconstants.InternalNode] = "true"

	if err := rt.PullImage(ctx, cfg.Image, clabtypes.PullPolicyIfNotPresent); err != nil {
		return fmt.Errorf("pulling Tailscale proxy image: %w", err)
	}
	if err := c.AllocateToolManagementIPs(ctx, cfg); err != nil {
		return fmt.Errorf("allocating Tailscale proxy management address: %w", err)
	}

	log.Info("Creating Tailscale SSO proxy", "container", cfg.LongName)
	id, err := rt.CreateContainer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("creating Tailscale proxy: %w", err)
	}
	if _, err := rt.StartContainer(ctx, id, clabruntime.NewEndpointlessNode(cfg)); err != nil {
		rt.DeleteContainer(ctx, cfg.LongName)
		return fmt.Errorf("starting Tailscale proxy: %w", err)
	}

	log.Info("Tailscale SSO proxy started; if it has not joined the tailnet yet, "+
		"open the login URL shown in its logs",
		"container", cfg.LongName, "logs", "docker logs "+cfg.LongName)

	return nil
}

// planTailscaleSidecarRecreates recreates the sidecars of recreated parents so that
// TS_DEST_IP follows the parent's management address.
func (c *CLab) planTailscaleSidecarRecreates(plan *applyPlan) {
	if plan == nil {
		return
	}

	parents := make([]string, 0, len(plan.recreatedNodeSet))
	for name := range plan.recreatedNodeSet {
		parents = append(parents, name)
	}
	for _, parent := range parents {
		name := clabnodestailscale.SidecarName(parent)
		sidecar, ok := c.Nodes[name]
		if !ok || sidecar.Config().Labels[clabconstants.RootNodeName] != parent {
			continue
		}
		if _, recreated := plan.recreatedNodeSet[name]; recreated {
			continue
		}
		if _, added := plan.addedNodeSet[name]; added {
			continue
		}

		plan.recreatedNodeSet[name] = struct{}{}
		delete(plan.restartNodeSet, name)
		delete(plan.linkRestartNodeSet, name)
		plan.nodeChangeReasons[name] = fmt.Sprintf("Tailscale parent %q recreated", parent)
	}
}

func (c *CLab) nodeRuntime(name string) string {
	n := c.Nodes[name]
	if n != nil && n.Config() != nil && n.Config().Runtime != "" {
		return n.Config().Runtime
	}

	return c.globalRuntimeName
}

func (c *CLab) addTailscaleNode(
	name, runtime string,
	labels map[string]string,
	waitFor []string,
) error {
	_, inNodes := c.Nodes[name]
	_, inTopo := c.Config.Topology.Nodes[name]
	if inNodes || inTopo {
		return fmt.Errorf(
			"cannot inject Tailscale sidecar %q: a node with that name already exists",
			name,
		)
	}

	def := &clabtypes.NodeDefinition{
		Kind:          clabnodestailscale.KindName,
		Image:         clabnodestailscale.DefaultImage,
		RestartPolicy: "always",
		Labels:        labels,
	}
	if len(waitFor) > 0 {
		wf := make(clabtypes.WaitForList, 0, len(waitFor))
		for _, node := range waitFor {
			wf = append(wf, &clabtypes.WaitFor{
				Node:  node,
				Stage: clabtypes.WaitForCreate,
			})
		}
		def.Stages = &clabtypes.Stages{
			Create: &clabtypes.StageCreate{
				StageBase: clabtypes.StageBase{WaitFor: wf},
			},
		}
	}
	c.Config.Topology.Nodes[name] = def

	if err := c.NewNode(name, runtime, def, len(c.Nodes)); err != nil {
		return fmt.Errorf("creating Tailscale sidecar %q: %w", name, err)
	}

	return nil
}
