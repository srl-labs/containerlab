package sonic_vm

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestInitSetsMacAddressForStartupConfig(t *testing.T) {
	node := new(sonic_vm)
	cfg := &clabtypes.NodeConfig{
		LabDir:    t.TempDir(),
		ShortName: "sonic",
	}

	if err := node.Init(cfg, clabnodes.WithMgmtNet(&clabtypes.MgmtNet{})); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if cfg.MacAddress == "" {
		t.Fatal("Init() did not set MacAddress")
	}
	if _, err := net.ParseMAC(cfg.MacAddress); err != nil {
		t.Fatalf("Init() set invalid MacAddress %q: %v", cfg.MacAddress, err)
	}

	dst := filepath.Join(t.TempDir(), "config_db.json")
	template := `{"DEVICE_METADATA":{"localhost":{"mac":"{{ .MacAddress }}"}}}`
	if err := node.GenerateConfig(dst, template); err != nil {
		t.Fatalf("GenerateConfig() failed: %v", err)
	}

	rendered, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read rendered config: %v", err)
	}
	if !strings.Contains(string(rendered), `"mac":"`+cfg.MacAddress+`"`) {
		t.Fatalf("rendered config does not contain MacAddress %q: %s", cfg.MacAddress, rendered)
	}
}

func TestBuildSSHKeyInjectionCommands(t *testing.T) {
	cmds := buildSSHKeyInjectionCommands([]string{
		"ssh-ed25519 AAAA test@host",
		"ssh-rsa BBBB comment-with-'quote",
	})

	for _, expected := range []string{
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh",
		"truncate -s 0 ~/.ssh/authorized_keys",
		"printf '%s\\n' 'ssh-ed25519 AAAA test@host' >> ~/.ssh/authorized_keys",
		"printf '%s\\n' 'ssh-rsa BBBB comment-with-'\\''quote' >> ~/.ssh/authorized_keys",
		"chmod 600 ~/.ssh/authorized_keys",
	} {
		found := false
		for _, got := range cmds {
			if got == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected command %q missing from %+v", expected, cmds)
		}
	}
}
