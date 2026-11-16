package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	clablinks "github.com/srl-labs/containerlab/links"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// bridgeNSMigParentNode resolves the namespace host of a migrated bridge-side
// endpoint. Stale refs carry either a live bridge node (resolved via the
// topology) or the name-encoded <bridge>|<parent> form of a node that is
// already removed from the desired topology.
func (c *CLab) bridgeNSMigParentNode(ref applyEndpointRef) (clablinks.Node, error) {
	if parentNode, err := c.bridgeNSParentNode(ref.key.node); err == nil {
		return parentNode, nil
	}
	parts := strings.SplitN(ref.key.node, "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("cannot resolve namespace host of bridge %q", ref.key.node)
	}
	node, exists := c.Nodes[parts[1]]
	if !exists {
		return nil, fmt.Errorf(
			"bridge %q: parent container %q not found",
			ref.key.node,
			parts[1],
		)
	}
	return node, nil
}

// parkMigratedBridgeNSPorts parks bridge-side veth ends whose owning in-container
// bridge was replaced by a bridge under a different parent (the move case).
// The veth pair stays intact: only the parked half leaves the dying parent's
// namespace, so the client-side end keeps its state untouched.
func (c *CLab) parkMigratedBridgeNSPorts(
	ctx context.Context,
	plan *applyPlan,
) error {
	if plan == nil || len(plan.bridgeNSMigrations) == 0 {
		return nil
	}

	for i := range plan.bridgeNSMigrations {
		mig := &plan.bridgeNSMigrations[i]

		if mig.relocation != nil {
			continue
		}

		// The old bridge node may already be deleted from the desired topology
		// (the move case); resolve its namespace host by the name-encoded
		// parent suffix, falling back to the normal resolution.
		parentNode, err := c.bridgeNSMigParentNode(mig.staleRef)
		if err != nil {
			return err
		}

		newNode, exists := c.Nodes[mig.newKey.node]
		if !exists {
			return fmt.Errorf("bridge migration target %q not found", mig.newKey.node)
		}
		longName := newNode.Config().LongName

		relocation, err := clablinks.ParkBridgePortByOwnership(
			ctx,
			parentNode,
			mig.staleRef.key.node,
			mig.staleRef.key.iface,
			longName,
		)
		if err != nil {
			return err
		}

		mig.relocation = relocation
	}

	return nil
}

// restoreMigratedBridgeNSPorts moves parked bridge-side ends into the replacement
// bridge's parent namespace after that container (and the bridge) exist again,
// relabels the ownership altname to the replacement bridge node, and re-enslaves
// the port to the bridge. The corresponding live link is marked full-deployed so
// deployLinks skips pair re-creation for it.
func (c *CLab) restoreMigratedBridgeNSPorts(
	ctx context.Context,
	plan *applyPlan,
) error {
	if plan == nil || len(plan.bridgeNSMigrations) == 0 {
		return nil
	}

	for i := range plan.bridgeNSMigrations {
		mig := &plan.bridgeNSMigrations[i]

		if mig.relocation == nil || plan.plannedLinkSet == nil {
			continue
		}

		// Restore into the replacement bridge's parent namespace; the bridge
		// node interfaces with the parent's ns through its GetNSPath override.
		bridgeNode, exists := c.Nodes[mig.newKey.node]
		if !exists {
			return fmt.Errorf("bridge migration target %q not found", mig.newKey.node)
		}

		if err := mig.relocation.RestoreBridgePort(
			ctx,
			bridgeNode,
			mig.newKey.node,
			bridgeNSNameInNamespace(mig.newKey.node),
			clabutils.SanitizeInterfaceName(mig.newKey.iface),
		); err != nil {
			return err
		}

		// The migrated pair is complete end-to-end (the client-side end was
		// never touched and the parked bridge-side half is restored and
		// enslaved). Filter the corresponding replaced link out of addedLinks
		// so endpoint removal and pair re-creation never touch it.
		kept := plan.addedLinks[:0]
		for _, link := range plan.addedLinks {
			if clablinks.RuntimeEndpointsContainEndpoint(
				link,
				mig.newKey.node,
				mig.newKey.iface,
			) {
				log.Info(
					"Restored migrated bridge port into replacement bridge",
					"bridge",
					mig.newKey.node,
					"port",
					mig.newKey.iface,
				)
				continue
			}
			kept = append(kept, link)
		}
		plan.addedLinks = kept
	}

	return nil
}
