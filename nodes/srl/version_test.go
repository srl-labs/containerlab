package srl

import (
	"bytes"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
)

func TestParseVersionString(t *testing.T) {
	n := &srl{}

	tests := map[string]struct {
		s    string
		want *SrlVersion
	}{
		"valid version string": {
			s:    "v24.3.1-154-gffc27e28d7",
			want: &SrlVersion{"24", "3", "1", "154", "gffc27e28d7"},
		},
		"valid beta version string": {
			s:    "v24.7.1-330-gffc27e28d7-dirty",
			want: &SrlVersion{"24", "7", "1", "330", "gffc27e28d7-dirty"},
		},
		"invalid version string": {
			s:    "invalid",
			want: &SrlVersion{"0", "0", "0", "0", "0"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := n.parseVersionString(tt.s)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatalf("parseVersionString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSrlVersionString(t *testing.T) {
	tests := map[string]struct {
		v    *SrlVersion
		want string
	}{
		"all fields filled": {
			v:    &SrlVersion{"24", "3", "1", "154", "gffc27e28d7"},
			want: "v24.3.1-154-gffc27e28d7",
		},
		"all fields empty": {
			v:    &SrlVersion{"0", "0", "0", "0", "0"},
			want: "v0.0.0-0-0",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("SrlVersion.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMajorMinorSemverString(t *testing.T) {
	tests := map[string]struct {
		v    *SrlVersion
		want string
	}{
		"all fields filled": {
			v:    &SrlVersion{"24", "3", "1", "154", "gffc27e28d7"},
			want: "v24.3",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.v.MajorMinorSemverString(); got != tt.want {
				t.Errorf("SrlVersion.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

type testSSHPubKey struct {
	id byte
}

func (k testSSHPubKey) Type() string {
	return "ssh-test"
}

func (k testSSHPubKey) Marshal() []byte {
	return []byte{k.id}
}

func (k testSSHPubKey) Verify(_ []byte, _ *ssh.Signature) error {
	return nil
}

func testSSHPubKeys(count int) []ssh.PublicKey {
	keys := make([]ssh.PublicKey, count)
	for i := range keys {
		keys[i] = testSSHPubKey{id: byte(i)}
	}

	return keys
}

func TestSetVersionSpecificParamsSSHPubKeys(t *testing.T) {
	tests := map[string]struct {
		keyCount  int
		wantCount int
	}{
		"zero keys": {
			keyCount:  0,
			wantCount: 0,
		},
		"fewer than max": {
			keyCount:  3,
			wantCount: 3,
		},
		"exactly max": {
			keyCount:  srlMaxSSHPubKeys,
			wantCount: srlMaxSSHPubKeys,
		},
		"more than max": {
			keyCount:  46,
			wantCount: srlMaxSSHPubKeys,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			keys := testSSHPubKeys(tt.keyCount)
			n := &srl{
				sshPubKeys: keys,
				swVersion:  &SrlVersion{Major: "23", Minor: "10", Patch: "1", Build: "1", Commit: "test"},
			}
			n.Cfg = &clabtypes.NodeConfig{ShortName: "srl1"}

			tplData := &srlTemplateData{}
			if err := n.setVersionSpecificParams(tplData); err != nil {
				t.Fatalf("setVersionSpecificParams() error = %v", err)
			}

			want := clabutils.MarshalAndCatenateSSHPubKeys(wantSSHPubKeys(keys))
			if diff := cmp.Diff(want, tplData.SSHPubKeys); diff != "" {
				t.Fatalf("SSHPubKeys mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestSetVersionSpecificParamsSSHPubKeysDeterministic verifies that when more
// than the limit of keys are present, the retained subset does not depend on
// the order the keys were discovered in (RetrieveSSHPubKeys gathers them from a
// map, so the input order is non-deterministic).
func TestSetVersionSpecificParamsSSHPubKeysDeterministic(t *testing.T) {
	keys := testSSHPubKeys(46)
	reversed := make([]ssh.PublicKey, len(keys))
	for i := range keys {
		reversed[len(keys)-1-i] = keys[i]
	}

	render := func(in []ssh.PublicKey) string {
		n := &srl{
			sshPubKeys: in,
			swVersion:  &SrlVersion{Major: "23", Minor: "10", Patch: "1", Build: "1", Commit: "test"},
		}
		n.Cfg = &clabtypes.NodeConfig{ShortName: "srl1"}

		tplData := &srlTemplateData{}
		if err := n.setVersionSpecificParams(tplData); err != nil {
			t.Fatalf("setVersionSpecificParams() error = %v", err)
		}

		return tplData.SSHPubKeys
	}

	if got, want := render(keys), render(reversed); got != want {
		t.Fatalf("SSHPubKeys not deterministic across input orderings:\n got: %s\nwant: %s", got, want)
	}
}

// wantSSHPubKeys mirrors setVersionSpecificParams: it returns the keys verbatim
// when at or below the limit, and the deterministically sorted first
// srlMaxSSHPubKeys keys when above it.
func wantSSHPubKeys(keys []ssh.PublicKey) []ssh.PublicKey {
	if len(keys) <= srlMaxSSHPubKeys {
		return keys
	}

	sorted := make([]ssh.PublicKey, len(keys))
	copy(sorted, keys)
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(
			ssh.MarshalAuthorizedKey(sorted[i]),
			ssh.MarshalAuthorizedKey(sorted[j]),
		) < 0
	})

	return sorted[:srlMaxSSHPubKeys]
}
