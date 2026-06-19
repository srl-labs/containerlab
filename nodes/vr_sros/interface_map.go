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
	`^(?P<slot>\d+)/(?:x(?P<xiom>\d+)/)?(?P<mda>\d+)/(?P<port>\d+)$`,
)

const componentInterfaceHelp = "<slot>/<mda>/<port> or <slot>/x<xiom>/<mda>/<port> " +
	"(e.g. 2/1/3 or 1/x1/2/3), or ethX"

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

	lcs := make([]*clabtypes.Component, 0, len(components))
	for _, c := range components {
		if !isCPMSlot(c.Slot) {
			lcs = append(lcs, c)
		}
	}
	slices.SortFunc(lcs, func(a, b *clabtypes.Component) int {
		as, _ := strconv.Atoi(strings.TrimSpace(a.Slot))
		bs, _ := strconv.Atoi(strings.TrimSpace(b.Slot))
		return as - bs
	})

	base := 0
	var target *clabtypes.Component
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

	within, err := withinCardOffset(target, xiom, mda)
	if err != nil {
		return 0, fmt.Errorf("interface %q: %w", ifName, err)
	}

	return base + within + port, nil
}

// withinCardOffset returns the number of ethX ports that precede the requested (xiom, mda) within a
// single line card. Ordering: direct MDAs (ascending slot) first, then each XIOM (ascending slot)
// with its MDAs (ascending slot).
func withinCardOffset(c *clabtypes.Component, xiom, mda int) (int, error) {
	offset := 0

	directMDA := slices.Clone(c.MDA)
	slices.SortFunc(directMDA, func(a, b clabtypes.MDA) int { return a.Slot - b.Slot })
	for _, m := range directMDA {
		if xiom == 0 && m.Slot == mda {
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
			if xiom == x.Slot && m.Slot == mda {
				return offset, nil
			}
			offset += mdaPortCount(m.Type)
		}
	}

	return 0, fmt.Errorf("mda %d (xiom %d) is not present in the slot %s card", mda, xiom, c.Slot)
}
