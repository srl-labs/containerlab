// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
	"golang.org/x/crypto/ssh"
)

// startSSHServer serves password-authenticated exec sessions on 127.0.0.1 and answers
// each command with reply. It returns the address and the commands received.
func startSSHServer(
	t *testing.T,
	user, password string,
	reply func(cmd string) (out string, status uint32),
) (string, func() []string) {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
			if c.User() == user && string(p) == password {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("denied")
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	var mu sync.Mutex
	var cmds []string

	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_, chans, reqs, err := ssh.NewServerConn(nc, cfg)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for newCh := range chans {
					ch, chReqs, err := newCh.Accept()
					if err != nil {
						continue
					}
					for req := range chReqs {
						if req.Type != "exec" {
							_ = req.Reply(false, nil)
							continue
						}
						var p struct{ Command string }
						_ = ssh.Unmarshal(req.Payload, &p)
						_ = req.Reply(true, nil)
						mu.Lock()
						cmds = append(cmds, p.Command)
						mu.Unlock()
						out, status := reply(p.Command)
						_, _ = ch.Write([]byte(out))
						_, _ = ch.SendRequest("exit-status", false,
							ssh.Marshal(struct{ Status uint32 }{status}))
						ch.Close()
						break
					}
				}
			}()
		}
	}()

	return ln.Addr().String(), func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), cmds...)
	}
}

func TestExportConfigOverSSH(t *testing.T) {
	addr, received := startSSHServer(t, "admin", "pass word#1", func(string) (string, uint32) {
		return "\r\n# export\r\n/system identity\r\nset name=r1\r\n\r\n", 0
	})

	got, err := ExportConfigOverSSH(addr, "admin", "pass word#1", "/export")
	if err != nil {
		t.Fatalf("ExportConfigOverSSH: %v", err)
	}
	if got != "# export\r\n/system identity\r\nset name=r1" {
		t.Errorf("output = %q, want it trimmed", got)
	}
	if cmds := received(); len(cmds) != 1 || cmds[0] != "/export" {
		t.Errorf("commands run = %q, want [/export]", cmds)
	}
}

func TestExportConfigOverSSHErrors(t *testing.T) {
	tests := map[string]struct {
		password string
		out      string
		status   uint32
		want     string
	}{
		"wrong password": {password: "nope", out: "x", want: "failed to connect"},
		"empty output":   {password: "pw", out: " \r\n ", want: "empty configuration"},
		"command failed": {
			password: "pw",
			out:      "bad command",
			status:   1,
			want:     "failed to execute",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			addr, _ := startSSHServer(t, "admin", "pw", func(string) (string, uint32) {
				return tc.out, tc.status
			})

			_, err := ExportConfigOverSSH(addr, "admin", tc.password, "export")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestWriteSavedConfig(t *testing.T) {
	labDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(labDir, "ftpboot"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &clabtypes.NodeConfig{ShortName: "r1", LabDir: labDir}

	res, err := WriteSavedConfig(cfg, "ftpboot", "config.auto.rsc", []byte("set name=r1\n"))
	if err != nil {
		t.Fatalf("WriteSavedConfig: %v", err)
	}

	want := filepath.Join(labDir, "ftpboot", "config.auto.rsc")
	if res.ConfigPath != want {
		t.Errorf("ConfigPath = %q, want %q", res.ConfigPath, want)
	}
	if got, _ := os.ReadFile(want); string(got) != "set name=r1\n" {
		t.Errorf("written content = %q", got)
	}

	if _, err := WriteSavedConfig(cfg, "missing", "x.rsc", nil); err == nil {
		t.Error("want an error for a missing directory")
	}
}
