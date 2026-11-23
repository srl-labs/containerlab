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

// bridgeNSMigOwnerLongName returns the long name of the bridge node that owns
// the migration's stale endpoint: the desired node's configured long name when
// the bridge is part of the topology, and the standard "clab-<lab>-<node>"
// derivation for state-recovered bridges.
func (c *CLab) bridgeNSMigOwnerLongName(mig *bridgeNSMigration) string {
	if node, exists := c.Nodes[mig.staleRef.key.node]; exists {
		if cfg := node.Config(); cfg != nil && cfg.LongName != "" {
			return cfg.LongName
		}
	}
	return fmt.Sprintf("clab-%s-%s", c.Config.Name, mig.staleRef.key.node)
}

// parkMigratedBridgeNSPorts parks bridge-side veth ends whose owning in-container
// bridge was replaced by a bridge under a different parent (the move case).
// The veth pair stays intact: only the parked half leaves the dying parent's
// namespace, so the client-side end keeps its state untouched. Parking slots
// are keyed by the owning bridge's long name, so an interrupted migration's
// parked port stays attributable to its owner.
//
// State is modified incrementally: if an error occurs partway through,
// previously parked ports remain parked and the apply fails without
// invalidating their pairs. This is intentional; parking happens before any
// node is deleted, so the pre-apply state stays intact. Recovery happens
// automatically on the next apply: when the same move is planned again,
// parking finds the port parked and resumes instead of failing (the park step
// is idempotent), and when the next apply no longer plans the move,
// recoverParkedBridgeNSPorts plans the parked port's restore. Ports no
// desired link claims are swept from parking netnses at destroy.
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

		relocation, err := clablinks.ParkBridgePortByOwnership(
			ctx,
			parentNode,
			mig.staleRef.key.node,
			mig.staleRef.key.iface,
			c.bridgeNSMigOwnerLongName(mig),
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
// the port to the bridge. The migrated link is filtered out of addedLinks so
// deployLinks skips pair re-creation for it.
func (c *CLab) restoreMigratedBridgeNSPorts(
	ctx context.Context,
	plan *applyPlan,
) error {
	if plan == nil || len(plan.bridgeNSMigrations) == 0 {
		return nil
	}

	// Parked halves of every migration keep their parking netns until this
	// point; unlink those that ran out of ports even when a later restore
	// fails, so successful migrations do not leak onto failed applies.
	parkNSPaths := make(map[string]struct{})
	defer func() {
		for nsPath := range parkNSPaths {
			if err := clablinks.UnlinkEmptyParkingNetNS(nsPath); err != nil {
				log.Debug("failed to unlink empty parking netns",
					"path", nsPath, "err", err)
			}
		}
	}()

	for i := range plan.bridgeNSMigrations {
		mig := &plan.bridgeNSMigrations[i]

		if mig.relocation == nil || plan.plannedLinkSet == nil {
			continue
		}

		parkNSPaths[mig.relocation.ParkNSPath()] = struct{}{}

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

		// The moved bridge can leave its device deserted in a surviving
		// parent's namespace once all of its ports are restored elsewhere.
		if mig.staleRef.key.node != mig.newKey.node {
			c.collectDesertedBridgeDevice(plan, mig.staleRef.key.node)
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
					"Restored migrated bridge port",
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

// collectDesertedBridgeDevice records a bridge device for removal from its
// parent's namespace when the named in-container bridge has vacated it.
// Recovery migrations (stale and desired claim identical) restore into the
// owner and have no deserted side, so callers skip the record for them.
// Deleted or recreated parents clear their namespaces themselves.
func (c *CLab) collectDesertedBridgeDevice(plan *applyPlan, bridgeNodeName string) {
	parentName := c.extractBridgeParentName(bridgeNodeName)
	if parentName == "" {
		return
	}
	if _, deleted := plan.deletedNodeSet[parentName]; deleted {
		return
	}
	if _, recreated := plan.recreatedNodeSet[parentName]; recreated {
		return
	}
	// The recreated bridge keeps its kernel device name, so its replacement
	// must not be deleted.
	if _, recreated := plan.recreatedNodeSet[bridgeNodeName]; recreated {
		return
	}

	deviceName := bridgeNSNameInNamespace(bridgeNodeName)

	for i := range plan.desertedBridgeDevices {
		d := &plan.desertedBridgeDevices[i]
		if d.parentName == parentName && d.deviceName == deviceName {
			return
		}
	}

	plan.desertedBridgeDevices = append(plan.desertedBridgeDevices, desertedBridgeDevice{
		parentName: parentName,
		deviceName: deviceName,
	})
}

// planDesertedBridgeDeviceRemovals records the bridge devices of BridgeNS
// nodes dropped from the desired topology, when their parent container keeps
// running. No migration covers such removals: the bridge-side pair halves
// die together with the removed client-side ends, which leaves the bridge
// device behind as an empty husk in the parent. Runtime container names
// reject "|", so a "<bridge>|<parent>"-shaped node name uniquely identifies
// a BridgeNS node. Removed BridgeNS nodes never reach the container-based
// deleted-node scan (no container, not desired), so the previously deployed
// state's topology is their only reliable source of current identity.
func (c *CLab) planDesertedBridgeDeviceRemovals(plan *applyPlan) {
	for bridgeNodeName := range plan.deletedNodeSet {
		if !strings.Contains(bridgeNodeName, "|") {
			continue
		}
		c.collectDesertedBridgeDevice(plan, bridgeNodeName)
	}

	if plan.state == nil || plan.state.Topology == nil {
		return
	}
	for bridgeNodeName := range plan.state.Topology.Nodes {
		if _, desired := c.Nodes[bridgeNodeName]; desired {
			continue
		}
		if _, deleted := plan.deletedNodeSet[bridgeNodeName]; deleted {
			continue
		}
		if !strings.Contains(bridgeNodeName, "|") {
			continue
		}
		c.collectDesertedBridgeDevice(plan, bridgeNodeName)
	}
}

// removeDesertedBridgeDevices deletes the recorded deserted bridge devices
// from their parents' namespaces, repairing the state left by bridge moves
// and bridge-node removals whose parents keep running.
func (c *CLab) removeDesertedBridgeDevices(ctx context.Context, plan *applyPlan) error {
	for _, d := range plan.desertedBridgeDevices {
		parentNode, exists := c.Nodes[d.parentName]
		if !exists {
			continue
		}

		if err := clablinks.RemoveDesertedBridge(ctx, parentNode, d.deviceName); err != nil {
			log.Debug("Failed to remove deserted bridge device",
				"bridge", d.deviceName, "parent", d.parentName, "err", err)
		}
	}

	return nil
}
