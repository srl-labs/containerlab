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

var componentIfaceRegexp = regexp.MustCompile(
	`^(?P<slot>\d+)/(?:x(?P<xiom>\d+)/)?(?P<mda>\d+)/(?:c(?P<conn>\d+)/)?(?P<port>\d+)$`,
)

const componentInterfaceHelp = "<slot>/<mda>/<port>, <slot>/x<xiom>/<mda>/<port> or " +
	"<slot>/<mda>/c<conn>/<port> (e.g. 2/1/3, 1/x1/2/3 or 1/1/c1/1), or ethX"

type srosPortAlias struct {
	slot, xiom, mda, portIndex int
}

func parseSrosPortAlias(ifName string) (srosPortAlias, error) {
	groups, err := clabutils.GetRegexpCaptureGroups(componentIfaceRegexp, ifName)
	if err != nil {
		return srosPortAlias{}, err
	}

	p := srosPortAlias{}
	p.slot, _ = strconv.Atoi(groups["slot"])
	p.mda, _ = strconv.Atoi(groups["mda"])
	port, _ := strconv.Atoi(groups["port"])
	if groups["xiom"] != "" {
		p.xiom, _ = strconv.Atoi(groups["xiom"])
	}
	if port < 1 {
		return p, fmt.Errorf("interface %q has an invalid port number", ifName)
	}

	p.portIndex = port
	if groups["conn"] != "" {
		conn, _ := strconv.Atoi(groups["conn"])
		if port != 1 {
			return p, fmt.Errorf(
				"interface %q uses a breakout sub-port (c%d/%d); breakout ports are not auto-mapped, use the ethX name instead",
				ifName, conn, port,
			)
		}
		p.portIndex = conn
	}

	return p, nil
}

func (s *vrSROS) CalculateInterfaceIndex(ifName string) (int, error) {
	if len(s.Cfg.Components) == 0 {
		return simpleInterfaceIndex(ifName)
	}
	return componentInterfaceIndex(s.Cfg.Components, ifName)
}

func simpleInterfaceIndex(ifName string) (int, error) {
	p, err := parseSrosPortAlias(ifName)
	if err != nil {
		return 0, err
	}
	if p.slot != 1 || p.mda != 1 {
		return 0, fmt.Errorf(
			"interface %q cannot be mapped on a node without components: only slot 1 / mda 1 ports map automatically. "+
				"Define components: on this node to use XIOM/multi-MDA/multi-slot port names (e.g. 1/x1/2/c24/1 or 4/1/c1/1), or use the ethX name",
			ifName,
		)
	}
	return p.portIndex, nil
}

func componentInterfaceIndex(components []*clabtypes.Component, ifName string) (int, error) {
	p, err := parseSrosPortAlias(ifName)
	if err != nil {
		return 0, err
	}
	slot, xiom, mda, portIndex := p.slot, p.xiom, p.mda, p.portIndex

	lcs := make([]*clabtypes.Component, 0, len(components))
	for _, c := range components {
		if !isCPMSlot(c.Slot) {
			lcs = append(lcs, c)
		}
	}

	base := 0
	var target *clabtypes.Component
	if len(lcs) == 0 {
		if slot != 1 {
			return 0, fmt.Errorf("interface %q references slot %d, but integrated node only has data ports on slot 1", ifName, slot)
		}
		target = components[0]
	} else {
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

func withinCardOffset(c *clabtypes.Component, xiom, mda int) (int, error) {
	if len(c.MDA) == 0 && len(c.XIOM) == 0 {
		return 0, nil
	}

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
