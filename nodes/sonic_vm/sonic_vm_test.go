package sonic_vm

import "testing"

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
