package vr_sros

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// componentIfaceRegexp matches the SR OS port aliases used by components-based nodes:
//   - direct MDA:    <slot>/<mda>/<port>                e.g. 2/1/3
//   - XIOM MDA:      <slot>/x<xiom>/<mda>/<port>        e.g. 1/x1/2/3
//   - connectorized: <slot>/<mda>/c<conn>/<port>       e.g. 1/1/c1/1  (FP4/FP5 QSFP-DD cages)
//     (xiom + connector combine: <slot>/x<xiom>/<mda>/c<conn>/<port>)
var componentIfaceRegexp = regexp.MustCompile(
	`^(?P<slot>\d+)/(?:x(?P<xiom>\d+)/)?(?P<mda>\d+)/(?:c(?P<conn>\d+)/)?(?P<port>\d+)$`,
)

const componentInterfaceHelp = "<slot>/<mda>/<port>, <slot>/x<xiom>/<mda>/<port> or " +
	"<slot>/<mda>/c<conn>/<port> (e.g. 2/1/3, 1/x1/2/3 or 1/1/c1/1), or ethX"

func (s *vrSROS) CalculateInterfaceIndex(ifName string) (int, error) {
	if len(s.Cfg.Components) == 0 {
		return s.VRNode.DefaultNode.CalculateInterfaceIndex(ifName)
	}
	return componentInterfaceIndex(s.Cfg.Components, ifName)
}

func componentInterfaceIndex(components []*clabtypes.Component, ifName string) (int, error) {
	groups, err := clabutils.GetRegexpCaptureGroups(componentIfaceRegexp, ifName)
	if err != nil {
		return 0, err
	}

	slot, _ := strconv.Atoi(groups["slot"])
	mda, _ := strconv.Atoi(groups["mda"])
	port, _ := strconv.Atoi(groups["port"])
	xiom := 0
	if groups["xiom"] != "" {
		xiom, _ = strconv.Atoi(groups["xiom"])
	}
	if port < 1 {
		return 0, fmt.Errorf("interface %q has an invalid port number", ifName)
	}

	// portIndex is the 1-based position of the port within its MDA. On FP4/FP5 cards each connector
	// (cN) is one physical cage / one ethX, so the connector number is that position and the
	// trailing port is the breakout sub-port - which doesn't map to a distinct veth.
	portIndex := port
	if groups["conn"] != "" {
		conn, _ := strconv.Atoi(groups["conn"])
		if port != 1 {
			return 0, fmt.Errorf(
				"interface %q uses a breakout sub-port (c%d/%d); breakout ports are not auto-mapped, use the ethX name instead",
				ifName, conn, port,
			)
		}
		portIndex = conn
	}

	lcs := make([]*clabtypes.Component, 0, len(components))
	for _, c := range components {
		if !isCPMSlot(c.Slot) {
			lcs = append(lcs, c)
		}
	}

	base := 0
	var target *clabtypes.Component
	if len(lcs) == 0 {
		// integrated chassis: the (single) card sits in slot A / unset but its data ports are
		// addressed as slot 1 (e.g. sr-1, ixr-x). Map slot 1 to that card.
		if slot != 1 {
			return 0, fmt.Errorf("interface %q references slot %d, but integrated node only has data ports on slot 1", ifName, slot)
		}
		target = components[0]
	} else {
		// distributed chassis: line cards own contiguous ethX windows in ascending slot order.
		slices.SortFunc(lcs, func(a, b *clabtypes.Component) int {
			as, _ := strconv.Atoi(strings.TrimSpace(a.Slot))
			bs, _ := strconv.Atoi(strings.TrimSpace(b.Slot))
			return as - bs
		})
		for _, c := range lcs {
			cs, _ := strconv.Atoi(strings.TrimSpace(c.Slot))
			if cs == slot {
				target = c
				break
			}
			base += componentMaxNics(c)
		}
		if target == nil {
			return 0, fmt.Errorf("interface %q references slot %d which has no line card component", ifName, slot)
		}
	}

	within, err := withinCardOffset(target, xiom, mda)
	if err != nil {
		return 0, fmt.Errorf("interface %q: %w", ifName, err)
	}

	return base + within + portIndex, nil
}

// withinCardOffset returns the number of ethX ports that precede the requested (xiom, mda) within a
// single card. Ordering: direct MDAs (ascending slot) first, then each XIOM (ascending slot) with
// its MDAs (ascending slot).
//
// The XIOM in the alias is optional: SR OS may name an XIOM MDA's ports either with the xiom marker
// (1/x1/1/c1/1) or without it (1/1/c1/1). When the alias omits the xiom (xiom == 0) the MDA is
// matched by its number wherever it lives. Cards with no MDA list (e.g. an embedded IMM card such
// as cpm-ixr-x/imm6-...) have a single implicit port group, so the offset is 0.
func withinCardOffset(c *clabtypes.Component, xiom, mda int) (int, error) {
	if len(c.MDA) == 0 && len(c.XIOM) == 0 {
		return 0, nil
	}

	// matches reports whether the iterated MDA is the one the alias refers to. With an explicit
	// xiom the (xiom, mda) pair must match exactly; without one, the mda number alone matches.
	matches := func(isXiom bool, xiomSlot, mdaSlot int) bool {
		if xiom > 0 {
			return isXiom && xiomSlot == xiom && mdaSlot == mda
		}
		return mdaSlot == mda
	}

	offset := 0

	directMDA := slices.Clone(c.MDA)
	slices.SortFunc(directMDA, func(a, b clabtypes.MDA) int { return a.Slot - b.Slot })
	for _, m := range directMDA {
		if matches(false, 0, m.Slot) {
			return offset, nil
		}
		offset += mdaPortCount(m.Type)
	}

	xioms := slices.Clone(c.XIOM)
	slices.SortFunc(xioms, func(a, b clabtypes.XIOM) int { return a.Slot - b.Slot })
	for _, x := range xioms {
		xmda := slices.Clone(x.MDA)
		slices.SortFunc(xmda, func(a, b clabtypes.MDA) int { return a.Slot - b.Slot })
		for _, m := range xmda {
			if matches(true, x.Slot, m.Slot) {
				return offset, nil
			}
			offset += mdaPortCount(m.Type)
		}
	}

	return 0, fmt.Errorf("mda %d (xiom %d) is not present in the slot %s card", mda, xiom, c.Slot)
}
