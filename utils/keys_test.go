package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSSHPubKeysFromFiles(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Sample SSH public keys (these are test keys, not real private keys)
	validKey1 := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDf2YYb+xrDipxBLLS2jbKoE+Q+GA5mUVoarJjtXLXbgeL7A3Hb3q00ejfUR1CUwIuPe67t8zyPby5ZI/xz46svLWVCBzu1LN5tLxg8GhmFBHjv4Obc4616unuZ6QzKSxsrmimpk3tAgnQq6T9+9ReuqHIyoPAI/JhrZ0gY94BRaat/J6tA9FAZx4Co65JvY7KJhw1F689RbWno/WTJyd89MkA3fuNWuSCOqTedZ4QymT2ttcet8qHT03NuQ8TUcVzoiEW4xxPcUJn8e0Ps8zZsLM6Y5pCAWp4l3b+fOJme4HKOQSFtt0GuPN7CPk7k0tyG7s8sEup0luZUzjW4Ke9V"
	validKey2 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN0b9S0AkC8CfLprc/n2l4zUPxMVH2jmk5AE3IZxjqF0"

	// Create test files
	validKeyFile := filepath.Join(tmpDir, "valid_keys.pub")
	multiKeyFile := filepath.Join(tmpDir, "multi_keys.pub")
	emptyFile := filepath.Join(tmpDir, "empty.pub")
	commentedFile := filepath.Join(tmpDir, "commented.pub")

	tests := []struct {
		name      string
		setup     func() []string
		wantCount int
		wantErr   bool
		errStr    string
	}{
		{
			name: "valid_single_key",
			setup: func() []string {
				// Create file with one valid key
				content := validKey1 + " test@example.com\n"
				if err := os.WriteFile(validKeyFile, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return []string{validKeyFile}
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple_keys",
			setup: func() []string {
				// Create file with multiple keys
				content := validKey1 + " test1@example.com\n" +
					validKey2 + " test2@example.com\n"
				if err := os.WriteFile(multiKeyFile, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return []string{multiKeyFile}
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "empty_file",
			setup: func() []string {
				// Create empty file
				if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return []string{emptyFile}
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "file_with_comments",
			setup: func() []string {
				// Create file with comments and empty lines
				content := "# This is a comment\n\n" +
					validKey1 + " test@example.com\n" +
					"# Another comment\n" +
					validKey2 + " test2@example.com\n"
				if err := os.WriteFile(commentedFile, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return []string{commentedFile}
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "nonexistent_file",
			setup: func() []string {
				return []string{filepath.Join(tmpDir, "nonexistent.pub")}
			},
			wantCount: 0,
			wantErr:   true,
			errStr:    "no such file",
		},
		{
			name: "multiple_files",
			setup: func() []string {
				// Create two separate files
				file1 := filepath.Join(tmpDir, "file1.pub")
				file2 := filepath.Join(tmpDir, "file2.pub")

				if err := os.WriteFile(file1, []byte(validKey1+" test1@example.com\n"), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				if err := os.WriteFile(file2, []byte(validKey2+" test2@example.com\n"), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}

				return []string{file1, file2}
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := tt.setup()

			got, err := LoadSSHPubKeysFromFiles(paths)

			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadSSHPubKeysFromFiles() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
					t.Fatalf("expected error containing %q, got %q", tt.errStr, err.Error())
				}
				return
			}

			if len(got) != tt.wantCount {
				t.Fatalf("expected %d keys, got %d", tt.wantCount, len(got))
			}

			// Verify all returned items are valid SSH public keys
			for i, key := range got {
				if key == nil {
					t.Fatalf("key %d is nil", i)
				}
				// Try to marshal it back to ensure it's a valid key
				if key.Type() == "" {
					t.Fatalf("key %d has empty type", i)
				}
			}
		})
	}
}

func TestLoadSSHPubKeysFromFiles_InvalidKey(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file with invalid key
	invalidKeyFile := filepath.Join(tmpDir, "invalid.pub")
	invalidKeyContent := "ssh-rsa invalid-key-data test@example.com\n"

	if err := os.WriteFile(invalidKeyFile, []byte(invalidKeyContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err := LoadSSHPubKeysFromFiles([]string{invalidKeyFile})
	if err == nil {
		t.Fatalf("expected error for invalid key, got nil")
	}

	// Verify it's a parsing error from ssh package
	if !strings.Contains(err.Error(), "ssh:") &&
		!strings.Contains(strings.ToLower(err.Error()), "invalid") {
		t.Fatalf("expected SSH parsing error, got: %v", err)
	}
}

func TestLoadSSHPubKeysFromFiles_EmptyPaths(t *testing.T) {
	got, err := LoadSSHPubKeysFromFiles([]string{})
	if err != nil {
		t.Fatalf("unexpected error with empty paths: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected 0 keys for empty paths, got %d", len(got))
	}
}
