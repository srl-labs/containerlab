package nodes

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	clabtypes "github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
)

// KindSpecificConfigType is a kind's config type, registered with
// NodeRegistryEntryAttributes.WithKindSpecificConfig. Only KindSpecificConfigSpec implements it.
type KindSpecificConfigType interface {
	newKindSpecificConfig() any
}

// KindSpecificConfigSpec binds a kind to its config struct T, which holds the kind-specific keys of
// a node
// definition. A kind declares one spec, registers it and reads node configs through it:
//
//	var kindSpecificConfig clabnodes.KindSpecificConfigSpec[KindSpecificConfig]
//
//	nrea.WithKindSpecificConfig(kindSpecificConfig)
//	func (n *iol) kindSpecificCfg() *KindSpecificConfig { return kindSpecificConfig.Of(n.Cfg) }
//
// T must not implement yaml.Unmarshaler itself; its fields may.
type KindSpecificConfigSpec[T any] struct{}

// KindSpecificConfigDefaulter is implemented by kind-specific configs whose defaults are not the
// zero value.
// Topology keys decode on top of the defaults.
type KindSpecificConfigDefaulter interface {
	SetDefaults()
}

func (KindSpecificConfigSpec[T]) newKindSpecificConfig() any {
	return newDefaultKindSpecificConfig[T]()
}

func newDefaultKindSpecificConfig[T any]() *T {
	t := new(T)
	if d, ok := any(t).(KindSpecificConfigDefaulter); ok {
		d.SetDefaults()
	}

	return t
}

// Of returns cfg's kind-specific config, setting a default T when cfg has none (e.g. a node config
// built
// outside the topology). A kind-specific config of another type is a programming error and panics.
func (KindSpecificConfigSpec[T]) Of(cfg *clabtypes.NodeConfig) *T {
	switch kc := cfg.KindSpecificConfig.(type) {
	case *T:
		return kc
	case nil:
		t := newDefaultKindSpecificConfig[T]()
		cfg.KindSpecificConfig = t

		return t
	default:
		panic(
			fmt.Sprintf(
				"node %q: kind-specific config is %T, want %T",
				cfg.ShortName,
				kc,
				(*T)(nil),
			),
		)
	}
}

// InvalidKindSpecificConfig stands in for a kind-specific config that failed to decode, e.g. from a
// state file written before a key changed. ComputeDiff never considers it equal to another
// kind-specific config.
type InvalidKindSpecificConfig struct {
	Err string
}

// acceptsKey reports whether t has key. Strict decoding of `key: null` fails only for unknown
// fields, since yaml.v2 does not call field unmarshalers for null.
func acceptsKey(t KindSpecificConfigType, key string) bool {
	b, err := yaml.Marshal(map[string]any{key: nil})
	if err != nil {
		return false
	}

	return yaml.UnmarshalStrict(b, t.newKindSpecificConfig()) == nil
}

var yamlLinePrefix = regexp.MustCompile(`^line \d+: `)

// yamlErrorText returns a yaml.v2 error without its line prefixes, which refer to the
// re-marshalled key rather than to the topology file.
func yamlErrorText(err error) string {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err.Error()
	}

	msgs := make([]string, 0, len(typeErr.Errors))
	for _, e := range typeErr.Errors {
		msgs = append(msgs, yamlLinePrefix.ReplaceAllString(e, ""))
	}

	return strings.Join(msgs, "; ")
}

// DecodeKindSpecificConfig strictly decodes a node's raw kind-specific config entries into a new
// config of the
// kind registered in e. It returns a pointer to the kind's config type, or nil for kinds without
// a kind-specific config. Every unsupported key and invalid value is reported, naming the block it
// was set
// in.
func DecodeKindSpecificConfig(
	e *NodeRegistryEntry,
	node, kind string,
	entries []clabtypes.KindSpecificConfigEntry,
) (any, error) {
	t := e.KindSpecificConfigType()

	var kindSpecificConfig any
	if t != nil {
		kindSpecificConfig = t.newKindSpecificConfig()
	}

	var errs []error

	for _, en := range entries {
		if t == nil || !acceptsKey(t, en.Key) {
			errs = append(errs, fmt.Errorf("node %q: kind %q does not support key %q (set in %s)",
				node, kind, en.Key, en.From))

			continue
		}

		b, err := yaml.Marshal(map[string]any{en.Key: en.Value})
		if err == nil {
			err = yaml.UnmarshalStrict(b, kindSpecificConfig)
		}

		if err != nil {
			errs = append(errs, fmt.Errorf("node %q: invalid value for key %q (set in %s): %s",
				node, en.Key, en.From, yamlErrorText(err)))
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return kindSpecificConfig, nil
}
