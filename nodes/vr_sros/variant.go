package vr_sros

import (
	"fmt"
	"strings"

	clabtypes "github.com/srl-labs/containerlab/types"
)

func isCPMSlot(slot string) bool {
	s := strings.ToUpper(strings.TrimSpace(slot))
	return s == "" || s == "A" || s == "B"
}

func buildSrosVariant(chassis string, components []*clabtypes.Component, env map[string]string) (string, error) {
	// vrnetlab vsim currently only supports single CPM
	cpms := 0
	for _, c := range components {
		if isCPMSlot(c.Slot) {
			cpms++
		}
	}
	if cpms > 1 {
		return "", fmt.Errorf("kind nokia_sros (vSIM) only supports a single CPM card, found multiple defined.")
	}

	distributed := len(components) > 1
	for _, c := range components {
		if !isCPMSlot(c.Slot) {
			distributed = true
		}
	}

	// fetch any global SFM env var on the base node
	sfm := strings.TrimSpace(env["NOKIA_SROS_SFM"])

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

func componentTimosLine(chassis string, c *clabtypes.Component, sfm string) string {
	var parts []string

	// fetch cpu/ram/max_nics from env: under slot:
	if v := strings.TrimSpace(c.Env["cpu"]); v != "" {
		parts = append(parts, "cpu="+v)
	}
	if v := strings.TrimSpace(c.Env["ram"]); v != "" {
		parts = append(parts, "ram="+v)
	}
	if v := strings.TrimSpace(c.Env["max_nics"]); v != "" {
		parts = append(parts, "max_nics="+v)
	}

	parts = append(parts, "chassis="+chassis)

	slot := strings.TrimSpace(c.Slot)
	if slot == "" {
		// default to slot A for integrated components: method
		slot = "A"
	}
	parts = append(parts, "slot="+slot)

	// support slot defined sfm
	// else fallback to SRSIM env var
	// NOKIA_SROS_SFM.
	if c.SFM != "" {
		parts = append(parts, "sfm="+c.SFM)
	} else if sfm != "" {
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
