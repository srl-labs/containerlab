package types

type Component struct {
	Slot string            `yaml:"slot,omitempty"`
	Type string            `yaml:"type,omitempty"`
	Env  map[string]string `yaml:"env,omitempty"`
}

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
	}
}
