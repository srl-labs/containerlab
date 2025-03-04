package types

import (
	"fmt"
	"strings"
)

// Volume represents a volume mount specification in short syntax form.
// It mirrors the bind syntax but is limited to Docker-managed volumes.
type Volume struct {
	src  string
	dst  string
	mode string
}

// NewVolumeFromString parses a volume specification in the short syntax:
// - "/container/path"
// - "name:/container/path"
// - "name:/container/path:options"
func NewVolumeFromString(volume string) (*Volume, error) {
	v := &Volume{}

	split := strings.Split(volume, ":")

	if len(split) == 1 && split[0] != "" {
		// Anonymous volume, only destination is provided.
		v.dst = split[0]
		return v, nil
	}

	if strings.HasPrefix(split[0], "/") {
		return nil, fmt.Errorf("volume %q references a host path; please use the binds stanza instead", volume)
	} else if len(split) < 2 || len(split) > 3 {
		return nil, fmt.Errorf("unable to parse volume %q", volume)
	}

	v.src = split[0]
	v.dst = split[1]

	if len(split) == 3 { //nolint: mnd
		v.mode = split[2]
	}

	return v, nil
}

// Src returns the source (named volume) portion.
func (v *Volume) Src() string {
	return v.src
}

// Dst returns the destination path inside the container.
func (v *Volume) Dst() string {
	return v.dst
}

// Mode returns the raw option string.
func (v *Volume) Mode() string {
	return v.mode
}

// Options returns the option list split by comma.
func (v *Volume) Options() []string {
	if v.mode == "" {
		return nil
	}
	return strings.Split(v.mode, ",")
}

// String renders the volume back to its short syntax form.
func (v *Volume) String() string {
	s := v.dst

	if v.src != "" {
		s = v.src + ":" + s
	}

	if v.mode != "" {
		s += ":" + v.mode
	}

	return s
}

// VolumeOptions captures parsed options from the short syntax.
type VolumeOptions struct {
	ReadOnly bool
	NoCopy   bool
	Unknown  []string
}

// ParseVolumeOptions parses volume options from the short syntax.
func ParseVolumeOptions(opts []string) VolumeOptions {
	parsed := VolumeOptions{}

	for _, opt := range opts {
		switch opt {
		case "ro":
			parsed.ReadOnly = true
		case "rw", "":
			// ignore
		case "nocopy", "volume-nocopy":
			parsed.NoCopy = true
		default:
			parsed.Unknown = append(parsed.Unknown, opt)
		}
	}

	return parsed
}
