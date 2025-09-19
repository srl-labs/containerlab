package types

import "fmt"

type Component struct {
	Slot string            `yaml:"slot,omitempty"`
	Type string            `yaml:"type,omitempty"`
	Env  map[string]string `yaml:"env,omitempty"`
	SFM  string            `yaml:"sfm,omitempty"`
	XIOM XIOMS             `yaml:"xiom,omitempty"`
	MDA  MDAS              `yaml:"mda,omitempty"`
}

type XIOM struct {
	Slot int    `yaml:"slot,omitempty"`
	Type string `yaml:"type,omitempty"`
	MDA  MDAS   `yaml:"mda,omitempty"`
}

type XIOMS []XIOM

type MDA struct {
	Slot int    `yaml:"slot,omitempty"`
	Type string `yaml:"type,omitempty"`
}

type MDAS []MDA

func (c *Component) Copy() *Component {
	if c == nil {
		return nil
	}

	// Deep copy the map
	var envCopy map[string]string
	if c.Env != nil {
		envCopy = make(map[string]string, len(c.Env))
		for k, v := range c.Env {
			envCopy[k] = v
		}
	}

	return &Component{
		Slot: c.Slot,
		Type: c.Type,
		Env:  envCopy,
		SFM:  c.SFM,
		XIOM: c.XIOM.Copy(),
		MDA:  c.MDA.Copy(),
	}
}

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
			return fmt.Errorf("invalid mda entry. slot and type are required, got slot %q, type%q", e.Slot, e.Type)
		}
		if _, exists := slots[e.Slot]; exists {
			return fmt.Errorf("invalid mda entry. duplicate slot %d", e.Slot)
		}
		slots[e.Slot] = struct{}{}
	}

	*l = MDAS(entries)
	return nil
}

func (l MDAS) Copy() MDAS {
	if l == nil {
		return nil
	}
	out := make([]MDA, len(l))
	copy(out, l)
	return out
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
			return fmt.Errorf("invalid xiom entry. slot and type are required, got slot %q, type %q", e.Slot, e.Type)
		}
		if _, exists := slots[e.Slot]; exists {
			return fmt.Errorf("invalid xiom entry. duplicate slot %d", e.Slot)
		}
		slots[e.Slot] = struct{}{}
	}

	*l = XIOMS(entries)
	return nil
}

func (l XIOMS) Copy() XIOMS {
	if l == nil {
		return nil
	}
	out := make([]XIOM, len(l))
	copy(out, l)
	// deep copy
	for i := range out {
		out[i].MDA = out[i].MDA.Copy()
	}
	return out
}
