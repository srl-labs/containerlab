package types

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

func TestNodeDefinitionYAML_CredentialsNested(t *testing.T) {
	t.Parallel()
	var n NodeDefinition
	err := yaml.Unmarshal([]byte(`
credentials:
  username: u1
  password: p1
`), &n)
	if err != nil {
		t.Fatal(err)
	}
	if n.Credentials.Username != "u1" || n.Credentials.Password != "p1" {
		t.Fatalf("got %#v", n.Credentials)
	}
}

func TestNodeDefinitionYAML_LegacyFlatUsernamePassword(t *testing.T) {
	t.Parallel()
	var n NodeDefinition
	err := yaml.Unmarshal([]byte(`
username: legacy-u
password: legacy-p
`), &n)
	if err != nil {
		t.Fatal(err)
	}
	if n.Credentials.Username != "legacy-u" || n.Credentials.Password != "legacy-p" {
		t.Fatalf("expected legacy keys mapped into Credentials, got %#v", n.Credentials)
	}
}

func TestNodeDefinitionYAML_CredentialsWinsOverLegacy(t *testing.T) {
	t.Parallel()
	var n NodeDefinition
	err := yaml.Unmarshal([]byte(`
username: legacy-u
password: legacy-p
credentials:
  username: nested-u
  password: nested-p
`), &n)
	if err != nil {
		t.Fatal(err)
	}
	if n.Credentials.Username != "nested-u" || n.Credentials.Password != "nested-p" {
		t.Fatalf("nested credentials should take precedence, got %#v", n.Credentials)
	}
}

func TestNodeDefinitionYAML_KindSpecificConfigRoundTrip(t *testing.T) {
	t.Parallel()
	var n NodeDefinition
	err := yaml.UnmarshalStrict([]byte(`
kind: nokia_srsim
config-mode: classic
pid-offset: 5
`), &n)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"config-mode": "classic", "pid-offset": 5}
	if !reflect.DeepEqual(n.KindSpecificConfig, want) {
		t.Fatalf("KindSpecificConfig = %#v, want %#v", n.KindSpecificConfig, want)
	}

	b, err := yaml.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	var back NodeDefinition
	if err := yaml.UnmarshalStrict(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Kind != "nokia_srsim" || !reflect.DeepEqual(back.KindSpecificConfig, want) {
		t.Fatalf(
			"round trip got kind %q settings %#v from:\n%s",
			back.Kind,
			back.KindSpecificConfig,
			b,
		)
	}
}

func TestNodeDefinitionImportEnvsFrom(t *testing.T) {
	tests := map[string]struct {
		nodeEnv map[string]string
		env     []string
		want    map[string]string
	}{
		"import_env": {
			nodeEnv: map[string]string{importEnvsKey: "true", "KEEP": "node"},
			env:     []string{"GIVEN=value", "KEEP=env"},
			want:    map[string]string{importEnvsKey: "true", "KEEP": "node", "GIVEN": "value"},
		},
		"no_import_key": {
			nodeEnv: map[string]string{"KEEP": "node"},
			env:     []string{"GIVEN=value"},
			want:    map[string]string{"KEEP": "node"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := &NodeDefinition{Env: tc.nodeEnv}
			n.ImportEnvsFrom(tc.env)

			if d := cmp.Diff(tc.want, n.Env); d != "" {
				t.Fatalf("env vars mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestNodeDefinitionImportEnvsFromProcessEnv(t *testing.T) {
	t.Setenv("CLAB_TEST_PROCESS_VAR", "process")

	n := &NodeDefinition{Env: map[string]string{importEnvsKey: "true"}}
	n.ImportEnvsFrom(nil)

	if d := cmp.Diff("process", n.Env["CLAB_TEST_PROCESS_VAR"]); d != "" {
		t.Fatalf("imported env var mismatch (-want +got):\n%s", d)
	}
}
