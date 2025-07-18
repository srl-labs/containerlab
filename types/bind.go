package types

import (
	"fmt"
	"strings"
)

// Binds represent a list of bind mounts.
type Binds []*Bind

// ToStringSlice returns a slice of strings representing the bind mounts.
func (b Binds) ToStringSlice() []string {
	result := make([]string, 0, len(b))
	for _, bind := range b {
		result = append(result, bind.String())
	}
	return result
}

// Bind represents a bind mount.
type Bind struct {
	src  string
	dst  string
	mode string
}

// NewBind creates a new Bind.
func NewBind(src, dst, mode string) *Bind {
	return &Bind{
		src:  src,
		dst:  dst,
		mode: mode,
	}
}

// NewBindFromString creates a new Bind instance from the string representation.
func NewBindFromString(bind string) (*Bind, error) {
	b := &Bind{}

	split := strings.Split(bind, ":")
	if len(split) == 1 {
		// If there is only one part, the container runtime creates an anonymous
		// volume and mounts it on the given destination.
		b.dst = split[0]
		return b, nil
	}
	if len(split) < 2 || len(split) > 3 {
		return nil, fmt.Errorf("unable to parse bind %q", bind)
	}

	b.src = split[0]
	b.dst = split[1]

	if len(split) == 3 {
		b.mode = split[2]
	}

	return b, nil
}

// Src returns the source path of the bind mount.
func (b *Bind) Src() string {
	return b.src
}

// Dst returns the destination path of the bind mount.
func (b *Bind) Dst() string {
	return b.dst
}

// Mode returns the mode of the bind mount.
func (b *Bind) Mode() string {
	return b.mode
}

// String returns the bind mount as a string.
func (b *Bind) String() string {
	s := b.dst
	if b.src != "" {
		s = fmt.Sprintf("%s:%s", b.src, s)
	}
	if b.mode != "" {
		s += fmt.Sprintf(":%s", b.mode)
	}

	return s
}
