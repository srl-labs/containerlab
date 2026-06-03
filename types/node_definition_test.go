package types

import (
	"testing"

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
