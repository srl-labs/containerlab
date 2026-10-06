package sros

import (
	"fmt"
	"strings"
)

// Component is an SR-SIM card: a CPM (slot A or B) or a line card.
type Component struct {
	Slot string            `yaml:"slot,omitempty" json:"slot,omitempty"`
	Type string            `yaml:"type,omitempty" json:"type,omitempty"`
	Env  map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	XIOM XIOMS             `yaml:"xiom,omitempty" json:"xiom,omitempty"`
	MDA  MDAS              `yaml:"mda,omitempty" json:"mda,omitempty"`
}

func componentsBySlot(components []*Component) map[string]Component {
	m := make(map[string]Component, len(components))

	for i, c := range components {
		if c == nil {
			continue
		}

		norm := *c
		norm.Slot = strings.ToUpper(strings.TrimSpace(c.Slot))

		key := norm.Slot
		if key == "" {
			key = fmt.Sprintf("#%d", i)
		}

		m[key] = norm
	}

	return m
}

type XIOM struct {
	Slot int    `yaml:"slot,omitempty" json:"slot,omitempty"`
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
	MDA  MDAS   `yaml:"mda,omitempty" json:"mda,omitempty"`
}

type XIOMS []XIOM

type MDA struct {
	Slot int    `yaml:"slot,omitempty" json:"slot,omitempty"`
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
}

type MDAS []MDA

func (l *MDAS) UnmarshalYAML(unmarshal func(any) error) error {
	var entries []MDA
	if err := unmarshal(&entries); err != nil {
		return err
	}

	if len(entries) == 0 {
		*l = nil
		return nil
	}

	slots := map[int]struct{}{}

	for _, e := range entries {
		if e.Type == "" || e.Slot <= 0 {
			return fmt.Errorf(
				"invalid mda entry. slot and type are required, got slot %d, type %q",
				e.Slot,
				e.Type,
			)
		}

		if _, exists := slots[e.Slot]; exists {
			return fmt.Errorf("invalid mda entry. duplicate slot %d", e.Slot)
		}

		slots[e.Slot] = struct{}{}
	}

	*l = MDAS(entries)

	return nil
}

func (l *XIOMS) UnmarshalYAML(unmarshal func(any) error) error {
	var entries []XIOM

	if err := unmarshal(&entries); err != nil {
		return err
	}

	if len(entries) == 0 {
		*l = nil
		return nil
	}

	slots := map[int]struct{}{}

	for _, e := range entries {
		if e.Type == "" || e.Slot <= 0 {
			return fmt.Errorf(
				"invalid xiom entry. slot and type are required, got slot %d, type %q",
				e.Slot,
				e.Type,
			)
		}

		if _, exists := slots[e.Slot]; exists {
			return fmt.Errorf("invalid xiom entry. duplicate slot %d", e.Slot)
		}

		slots[e.Slot] = struct{}{}
	}

	*l = XIOMS(entries)

	return nil
}

func setComponentEnvVars(env map[string]string, c *Component) {
	if c.Type != "" {
		env[envNokiaSrosCard] = c.Type
	}

	for _, x := range c.XIOM {
		key := fmt.Sprintf("%s_X%d", envNokiaSrosXIOM, x.Slot)
		env[key] = x.Type
		// add the nested MDA
		for _, m := range x.MDA {
			key := fmt.Sprintf("%s_X%d_%d", envNokiaSrosMDA, x.Slot, m.Slot)
			env[key] = m.Type
		}
	}

	for _, m := range c.MDA {
		key := fmt.Sprintf("%s_%d", envNokiaSrosMDA, m.Slot)
		env[key] = m.Type
	}
}

func (n *sros) checkComponentSlotsConfig() error {
	// check Slots are unique
	componentNames := map[string]struct{}{}
	for _, component := range n.kindSpecificCfg().Components {
		// convert slot to upper
		slot := strings.ToUpper(component.Slot)
		// check if slot exists
		_, exists := componentNames[slot]
		if exists {
			return fmt.Errorf(
				"node %s slot %s duplicate definition",
				n.GetShortName(),
				component.Slot,
			)
		}
		// add to component names map
		componentNames[component.Slot] = struct{}{}

	}
	return nil
}
