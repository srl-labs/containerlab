package srl

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const netnsNodeType = "srl-netns"

// namespaceNode owns the network namespace of an SR Linux node, so that the dataplane links and
// the management address survive a restart of the SR Linux container.
type namespaceNode struct {
	clabnodes.DefaultNode
}

func (n *namespaceNode) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg
	n.StopSignal = clabtypes.SIGTERM

	for _, o := range opts {
		o(n)
	}

	return nil
}

// usesNetnsNode reports whether the node runs in the network namespace of a holder container.
// Nodes with a user-defined network mode keep the network namespace of their own container.
func (n *srl) usesNetnsNode() bool {
	return n.Cfg.NetworkMode == "" && !n.Cfg.IsRootNamespaceBased
}

func (n *srl) initNetnsNode() error {
	if !n.usesNetnsNode() {
		return nil
	}

	n.netnsNode = new(namespaceNode)

	return n.netnsNode.Init(n.newNetnsConfig(), clabnodes.WithRuntime(n.GetRuntime()))
}

func (n *srl) newNetnsConfig() *clabtypes.NodeConfig {
	labels := maps.Clone(n.Cfg.Labels)

	if labels == nil {
		labels = map[string]string{}
	}

	shortName := n.Cfg.ShortName + "-netns"
	labels[clabconstants.NodeName] = shortName

	longName := n.Cfg.LongName + "-netns"
	labels[clabconstants.LongName] = longName

	labels[clabconstants.NodeType] = netnsNodeType
	labels[clabconstants.InternalNode] = "true"

	labels[clabconstants.RootNodeName] = n.Cfg.ShortName
	labels[clabconstants.RootNodeLongName] = n.Cfg.LongName

	return &clabtypes.NodeConfig{
		ShortName:             shortName,
		LongName:              longName,
		Hostname:              n.Cfg.GetHostname(),
		Fqdn:                  n.Cfg.Fqdn,
		LabDir:                n.Cfg.LabDir,
		Index:                 n.Cfg.Index,
		Kind:                  n.Cfg.Kind,
		NodeType:              netnsNodeType,
		Image:                 n.Cfg.Image,
		ImagePullPolicy:       n.Cfg.ImagePullPolicy,
		RestartPolicy:         n.Cfg.RestartPolicy,
		Sysctls:               maps.Clone(n.Cfg.Sysctls),
		User:                  "0:0",
		Entrypoint:            "/bin/sh",
		Cmd:                   `-c "trap 'exit 0' TERM INT; sleep infinity & wait $!"`,
		Env:                   map[string]string{},
		PortBindings:          n.Cfg.PortBindings,
		PortSet:               n.Cfg.PortSet,
		MgmtNet:               n.Cfg.MgmtNet,
		MgmtIntf:              n.Cfg.MgmtIntf,
		MgmtIPv4Address:       n.Cfg.MgmtIPv4Address,
		MgmtIPv4AddressAuto:   n.Cfg.MgmtIPv4AddressAuto,
		MgmtIPv4PrefixLength:  n.Cfg.MgmtIPv4PrefixLength,
		MgmtIPv6Address:       n.Cfg.MgmtIPv6Address,
		MgmtIPv6AddressAuto:   n.Cfg.MgmtIPv6AddressAuto,
		MgmtIPv6PrefixLength:  n.Cfg.MgmtIPv6PrefixLength,
		MgmtIPv4Gateway:       n.Cfg.MgmtIPv4Gateway,
		MgmtIPv6Gateway:       n.Cfg.MgmtIPv6Gateway,
		MacAddress:            n.Cfg.MacAddress,
		Aliases:               n.Cfg.Aliases,
		DNS:                   n.Cfg.DNS,
		ExtraHosts:            n.Cfg.ExtraHosts,
		Labels:                labels,
		Runtime:               n.Cfg.Runtime,
		ResultingPortBindings: n.Cfg.ResultingPortBindings,
	}
}

// containerConfig returns the config of the SR Linux container attached to the holder's
// network namespace. The runtime rejects network settings for such containers, as they
// belong to the holder.
func (n *srl) containerConfig() *clabtypes.NodeConfig {
	cfg := *n.Cfg
	cfg.NetworkMode = "container:" + n.netnsNode.GetShortName()
	cfg.DNS = nil
	cfg.ExtraHosts = nil
	cfg.MacAddress = ""
	cfg.PortBindings = nil
	cfg.PortSet = nil

	return &cfg
}

func (n *srl) ensureNetnsRunning(ctx context.Context) error {
	switch status := n.netnsNode.GetContainerStatus(ctx); status {
	case clabruntime.Running:
	case clabruntime.Created, clabruntime.Stopped:
		if err := n.netnsNode.Start(ctx); err != nil {
			return fmt.Errorf("node %q network namespace container start error: %w",
				n.Cfg.ShortName, err)
		}
	case clabruntime.Paused:
		if err := n.Runtime.UnpauseContainer(ctx, n.netnsNode.Config().LongName); err != nil {
			return fmt.Errorf("node %q network namespace container unpause error: %w",
				n.Cfg.ShortName, err)
		}
	case clabruntime.NotFound:
		return fmt.Errorf(
			"node %q network namespace container %q not found; recreate the node",
			n.Cfg.ShortName, n.netnsNode.Config().LongName,
		)
	default:
		return fmt.Errorf("node %q network namespace container %q is %s",
			n.Cfg.ShortName, n.netnsNode.Config().LongName, status)
	}

	return nil
}

// linkNetns points the node's netns symlink to the holder, as the namespace path of the
// SR Linux container changes whenever the container restarts.
func (n *srl) linkNetns(ctx context.Context) error {
	nsPath, err := n.netnsNode.GetNSPath(ctx)
	if err != nil {
		return err
	}

	return clabutils.LinkContainerNS(nsPath, n.Cfg.LongName)
}

func (n *srl) Deploy(ctx context.Context, params *clabnodes.DeployParams) error {
	if n.netnsNode == nil {
		return n.DefaultNode.Deploy(ctx, params)
	}

	// network settings are finalized after Init, so the holder config is rebuilt here.
	*n.netnsNode.Config() = *n.newNetnsConfig()
	if err := n.netnsNode.Deploy(ctx, params); err != nil {
		return fmt.Errorf("deploy network namespace node %q: %w", n.netnsNode.GetShortName(), err)
	}

	n.Cfg.Env[clabconstants.ClabEnvIntfs] = strconv.Itoa(len(n.GetEndpoints()))

	cID, err := n.Runtime.CreateContainer(ctx, n.containerConfig())
	if err != nil {
		return err
	}

	if _, err := n.Runtime.StartContainer(ctx, cID, n); err != nil {
		return err
	}

	if err := n.linkNetns(ctx); err != nil {
		return err
	}

	n.SetState(clabnodesstate.Deployed)

	return nil
}

func (n *srl) Start(ctx context.Context) error {
	if n.netnsNode == nil {
		return n.DefaultNode.Start(ctx)
	}

	if err := n.ensureNetnsRunning(ctx); err != nil {
		return err
	}

	if err := n.DefaultNode.Start(ctx); err != nil {
		return err
	}

	return n.linkNetns(ctx)
}

// Stop also stops the holder, so that the next start gets a fresh network namespace without
// interfaces left behind by the stopped SR Linux container.
func (n *srl) Stop(ctx context.Context) error {
	if err := n.DefaultNode.Stop(ctx); err != nil || n.netnsNode == nil {
		return err
	}

	if n.netnsNode.GetContainerStatus(ctx) != clabruntime.Running {
		return nil
	}

	return n.Runtime.StopContainer(ctx, n.netnsNode.Config().LongName, n.netnsNode.StopSignal)
}

func (n *srl) Delete(ctx context.Context) error {
	if n.netnsNode == nil {
		return n.DefaultNode.Delete(ctx)
	}

	return errors.Join(n.DefaultNode.Delete(ctx), n.netnsNode.Delete(ctx))
}

// GetContainers returns the SR Linux container with the network settings of the holder.
// When only the holder exists, it is returned so that it is not orphaned on destroy.
func (n *srl) GetContainers(ctx context.Context) ([]clabruntime.GenericContainer, error) {
	cnts, err := n.DefaultNode.GetContainers(ctx)
	if n.netnsNode == nil {
		return cnts, err
	}

	if err != nil {
		if errors.Is(err, clabnodes.ErrContainersNotFound) {
			return n.netnsNode.GetContainers(ctx)
		}

		return nil, err
	}

	holder, err := n.netnsNode.GetContainers(ctx)
	if err != nil {
		if errors.Is(err, clabnodes.ErrContainersNotFound) {
			return cnts, nil
		}

		return nil, err
	}

	cnts[0].NetworkName = holder[0].NetworkName
	cnts[0].NetworkSettings = holder[0].NetworkSettings
	cnts[0].Ports = holder[0].Ports

	return cnts, nil
}

// GetNSPath retrieves the Namespace Path.
func (n *srl) GetNSPath(ctx context.Context) (string, error) {
	if n.netnsNode == nil {
		return n.DefaultNode.GetNSPath(ctx)
	}

	return n.netnsNode.GetNSPath(ctx)
}

// DeleteNetnsSymlink deletes the symlink file created for the container netns.
func (n *srl) DeleteNetnsSymlink() error {
	if n.netnsNode == nil {
		return n.DefaultNode.DeleteNetnsSymlink()
	}

	return errors.Join(n.DefaultNode.DeleteNetnsSymlink(), n.netnsNode.DeleteNetnsSymlink())
}
