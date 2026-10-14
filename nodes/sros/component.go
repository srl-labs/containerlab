package sros

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
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
		componentNames[slot] = struct{}{}

	}
	return nil
}

// componentDerivedEnv is the environment a component would set from its type, XIOM, MDA,
// and per-component env. Per-component env overrides the typed fields.
func componentDerivedEnv(c *Component) map[string]string {
	derived := map[string]string{}
	if c == nil {
		return derived
	}
	setComponentEnvVars(derived, c)
	maps.Copy(derived, c.Env)
	return derived
}

// warnUserEnvOverrides logs when a node env var replaces a component-derived value.
func warnUserEnvOverrides(node string, derived, userEnv map[string]string) {
	for key, componentValue := range derived {
		envValue, userSet := userEnv[key]
		if !userSet || componentValue == envValue {
			continue
		}
		log.Warn(
			fmt.Sprintf(
				"component value %q will not be used because environment variable %s is set",
				componentValue,
				key,
			),
			"node", node,
			"env", key,
			"component-value", componentValue,
			"env-value", envValue,
		)
	}
}

// keepUserEnv copies component-derived keys into dst unless the user already set that key.
// A user value wins, and a differing component value is logged.
func keepUserEnv(node string, dst, derived, userEnv map[string]string) {
	warnUserEnvOverrides(node, derived, userEnv)
	for key, componentValue := range derived {
		if envValue, userSet := userEnv[key]; userSet {
			dst[key] = envValue
			continue
		}
		dst[key] = componentValue
	}
}

// cloneComponent returns a deep copy of c suitable for env overrides during config generation.
func cloneComponent(c *Component) *Component {
	if c == nil {
		return &Component{}
	}
	out := *c
	out.Env = maps.Clone(c.Env)
	out.MDA = slices.Clone(c.MDA)
	if len(c.XIOM) > 0 {
		out.XIOM = make(XIOMS, len(c.XIOM))
		for i, xiom := range c.XIOM {
			xiom.MDA = slices.Clone(xiom.MDA)
			out.XIOM[i] = xiom
		}
	}
	return &out
}

// applyHardwareEnv overlays SR-SIM card, XIOM, and MDA environment variables onto c.
// NOKIA_SROS_SLOT is left unchanged; it identifies the container rather than provisioned hardware.
func applyHardwareEnv(c *Component, env map[string]string) {
	if c == nil || len(env) == 0 {
		return
	}
	if card := env[envNokiaSrosCard]; card != "" {
		c.Type = card
	}
	for key, val := range env {
		if val == "" {
			continue
		}
		switch {
		case strings.HasPrefix(key, envNokiaSrosMDA+"_X"):
			rest := strings.TrimPrefix(key, envNokiaSrosMDA+"_X")
			xiomStr, mdaStr, ok := strings.Cut(rest, "_")
			if !ok {
				continue
			}
			xiomSlot, errX := strconv.Atoi(xiomStr)
			mdaSlot, errM := strconv.Atoi(mdaStr)
			if errX != nil || errM != nil || xiomSlot <= 0 || mdaSlot <= 0 {
				continue
			}
			upsertXiomMDA(c, xiomSlot, mdaSlot, val)
		case strings.HasPrefix(key, envNokiaSrosMDA+"_"):
			slot, err := strconv.Atoi(strings.TrimPrefix(key, envNokiaSrosMDA+"_"))
			if err != nil || slot <= 0 {
				continue
			}
			c.MDA = upsertMDA(c.MDA, slot, val)
		case strings.HasPrefix(key, envNokiaSrosXIOM+"_X"):
			slot, err := strconv.Atoi(strings.TrimPrefix(key, envNokiaSrosXIOM+"_X"))
			if err != nil || slot <= 0 {
				continue
			}
			upsertXiomType(c, slot, val)
		}
	}
}

func upsertMDA(mdas MDAS, slot int, typ string) MDAS {
	for i := range mdas {
		if mdas[i].Slot == slot {
			mdas[i].Type = typ
			return mdas
		}
	}
	return append(mdas, MDA{Slot: slot, Type: typ})
}

func upsertXiomType(c *Component, slot int, typ string) {
	for i := range c.XIOM {
		if c.XIOM[i].Slot == slot {
			c.XIOM[i].Type = typ
			return
		}
	}
	c.XIOM = append(c.XIOM, XIOM{Slot: slot, Type: typ})
}

func upsertXiomMDA(c *Component, xiomSlot, mdaSlot int, typ string) {
	for i := range c.XIOM {
		if c.XIOM[i].Slot != xiomSlot {
			continue
		}
		c.XIOM[i].MDA = upsertMDA(c.XIOM[i].MDA, mdaSlot, typ)
		return
	}
	c.XIOM = append(c.XIOM, XIOM{
		Slot: xiomSlot,
		MDA:  MDAS{{Slot: mdaSlot, Type: typ}},
	})
}
