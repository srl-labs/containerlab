package core

import (
	"net"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestDescriptiveYamlError(t *testing.T) {
	typeErr := &yaml.TypeError{Errors: []string{
		`line 8: key "n1" already set in map`,
		`line 7: field bogus-field not found in type types.NodeDefinitionWithDeprecatedFields`,
		`line 3: cannot unmarshal !!str ` + "`abc`" + ` into int`,
	}}

	got := descriptiveYamlError(typeErr).Error()

	for _, want := range []string{
		`line 8: "n1" is defined more than once`,
		`line 7: unknown field "bogus-field"`,
		"line 3: cannot unmarshal", // untranslated messages pass through
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in error, got:\n%s", want, got)
		}
	}
}

func TestCheckPortAvailable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	addr := l.Addr().String()

	if err := checkPortAvailable("tcp", addr); err == nil {
		t.Errorf("expected error for busy port %s, got nil", addr)
	}

	l.Close()

	if err := checkPortAvailable("tcp", addr); err != nil {
		t.Errorf("expected no error for free port %s, got %v", addr, err)
	}

	// unsupported protocols are not probed
	if err := checkPortAvailable("sctp", addr); err != nil {
		t.Errorf("expected no error for sctp, got %v", err)
	}
}
