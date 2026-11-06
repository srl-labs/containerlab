package nodes

import (
	"context"
	"fmt"

	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// NetnsHolder is an internal container that owns the network namespace of a node. The node's
// containers join it, so the links and the management address outlive their restarts.
type NetnsHolder struct {
	DefaultNode
}

func (n *NetnsHolder) Init(cfg *clabtypes.NodeConfig, opts ...NodeOption) error {
	n.DefaultNode = *NewDefaultNode(n)
	n.Cfg = cfg
	n.StopSignal = clabtypes.SIGTERM

	for _, o := range opts {
		o(n)
	}

	return nil
}

// EnsureRunning starts or unpauses the holder, so that the containers of node can join it.
func (n *NetnsHolder) EnsureRunning(ctx context.Context, node string) error {
	switch status := n.GetContainerStatus(ctx); status {
	case clabruntime.Running:
	case clabruntime.Created, clabruntime.Stopped:
		if err := n.Start(ctx); err != nil {
			return fmt.Errorf("node %q network namespace container start error: %w", node, err)
		}
	case clabruntime.Paused:
		if err := n.Runtime.UnpauseContainer(ctx, n.Cfg.LongName); err != nil {
			return fmt.Errorf("node %q network namespace container unpause error: %w", node, err)
		}
	case clabruntime.NotFound:
		return fmt.Errorf(
			"node %q network namespace container %q not found; recreate the node",
			node, n.Cfg.LongName,
		)
	default:
		return fmt.Errorf("node %q network namespace container %q is %s",
			node, n.Cfg.LongName, status)
	}

	return nil
}
