package vr_sros

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func isCPMSlot(slot string) bool {
	s := strings.ToUpper(strings.TrimSpace(slot))
	return s == "" || s == "A" || s == "B"
}

func buildSrosVariant(
	chassis string,
	components []*Component,
	sfm string,
) (string, error) {
	// vrnetlab vsim currently only supports single CPM
	cpms := 0
	for _, c := range components {
		if isCPMSlot(c.Slot) {
			cpms++
		}
	}
	if cpms > 1 {
		return "", fmt.Errorf(
			"kind nokia_sros (vSIM) only supports a single CPM card, found multiple defined.",
		)
	}

	distributed := len(components) > 1
	for _, c := range components {
		if !isCPMSlot(c.Slot) {
			distributed = true
		}
	}

	if !distributed {
		return componentTimosLine(chassis, components[0], sfm), nil
	}

	segments := make([]string, 0, len(components))
	// order CPM first
	for _, c := range components {
		if isCPMSlot(c.Slot) {
			segments = append(segments, "cp: "+componentTimosLine(chassis, c, sfm))
		}
	}
	// then line cards
	for _, c := range components {
		if !isCPMSlot(c.Slot) {
			segments = append(segments, "lc: "+componentTimosLine(chassis, c, sfm))
		}
	}
	// delimit
	return strings.Join(segments, " ___ "), nil
}

func componentTimosLine(chassis string, c *Component, sfm string) string {
	var parts []string

	if c.CPU > 0 {
		parts = append(parts, "cpu="+strconv.Itoa(c.CPU))
	}
	if c.RAM > 0 {
		parts = append(parts, "ram="+strconv.Itoa(c.RAM))
	}
	if n := componentMaxNics(c); n > 0 {
		parts = append(parts, "max_nics="+strconv.Itoa(n))
	}

	parts = append(parts, "chassis="+chassis)

	slot := strings.TrimSpace(c.Slot)
	if slot == "" {
		// default to slot A for integrated components: method
		slot = "A"
	}
	parts = append(parts, "slot="+slot)

	if sfm != "" {
		parts = append(parts, "sfm="+sfm)
	}

	if c.Type != "" {
		parts = append(parts, "card="+c.Type)
	}

	for _, x := range c.XIOM {
		parts = append(parts, fmt.Sprintf("xiom/x%d=%s", x.Slot, x.Type))
		for _, m := range x.MDA {
			parts = append(parts, fmt.Sprintf("mda/x%d/%d=%s", x.Slot, m.Slot, m.Type))
		}
	}

	for _, m := range c.MDA {
		parts = append(parts, fmt.Sprintf("mda/%d=%s", m.Slot, m.Type))
	}

	return strings.Join(parts, " ")
}

func componentMaxNics(c *Component) int {
	if c.MaxNics > 0 {
		return c.MaxNics
	}
	return componentPortCount(c)
}

// figure out the max_nics per slot.
func componentPortCount(c *Component) int {
	total := 0
	for _, m := range c.MDA {
		total += mdaPortCount(m.Type)
	}
	for _, x := range c.XIOM {
		for _, m := range x.MDA {
			total += mdaPortCount(m.Type)
		}
	}
	return total
}

// get the first number from the mda type, delimit based on the '+'
// for mdas with multiple formfactors/speeds.
func mdaPortCount(mdaType string) int {
	total := 0
	for group := range strings.SplitSeq(mdaType, "+") {
		if m := regexp.MustCompile(`^[a-zA-Z]*(\d+)`).FindStringSubmatch(group); m != nil {
			n, _ := strconv.Atoi(m[1])
			total += n
		}
	}
	return total
}
