package core

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRelPathWithinDir(t *testing.T) {
	t.Parallel()

	base := filepath.Join("tmp", "lab")
	tests := []struct {
		name string
		path string
		base string
		want string
	}{
		{
			name: "under-base",
			path: filepath.Join(base, "node", "config.cfg"),
			base: base,
			want: filepath.Join("node", "config.cfg"),
		},
		{
			name: "outside-base",
			path: filepath.Join("other", "config.cfg"),
			base: base,
			want: "config.cfg",
		},
		{
			name: "empty-base",
			path: filepath.Join("somewhere", "config.cfg"),
			base: "",
			want: "config.cfg",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := relPathWithinDir(tt.path, tt.base)
			if got != tt.want {
				t.Fatalf("relPathWithinDir(%q, %q) = %q, want %q", tt.path, tt.base, got, tt.want)
			}
		})
	}
}

func TestSplitNameAndExt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		input             string
		wantBase          string
		wantExt           string
		wantAlreadyPacked bool
	}{
		{
			name:              "plain",
			input:             "config.cfg",
			wantBase:          "config",
			wantExt:           ".cfg",
			wantAlreadyPacked: false,
		},
		{
			name:              "tar-gz",
			input:             "config.tar.gz",
			wantBase:          "config",
			wantExt:           ".tar.gz",
			wantAlreadyPacked: true,
		},
		{
			name:              "gz",
			input:             "config.gz",
			wantBase:          "config",
			wantExt:           ".gz",
			wantAlreadyPacked: true,
		},
		{
			name:              "no-ext",
			input:             "config",
			wantBase:          "config",
			wantExt:           "",
			wantAlreadyPacked: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotBase, gotExt, gotPacked := splitNameAndExt(tt.input)
			if gotBase != tt.wantBase || gotExt != tt.wantExt || gotPacked != tt.wantAlreadyPacked {
				t.Fatalf(
					"splitNameAndExt(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.input,
					gotBase,
					gotExt,
					gotPacked,
					tt.wantBase,
					tt.wantExt,
					tt.wantAlreadyPacked,
				)
			}
		})
	}
}

func TestTimestampedFilePath(t *testing.T) {
	t.Parallel()

	ts := "240101_010101"
	got, compress := timestampedFilePath(filepath.Join("dir", "config.cfg"), ts)
	want := filepath.Join("dir", "config-240101_010101.cfg.gz")
	if got != want || !compress {
		t.Fatalf("timestampedFilePath plain = (%q, %v), want (%q, true)", got, compress, want)
	}

	got, compress = timestampedFilePath(filepath.Join("dir", "config.tar.gz"), ts)
	want = filepath.Join("dir", "config-240101_010101.tar.gz")
	if got != want || compress {
		t.Fatalf("timestampedFilePath packed = (%q, %v), want (%q, false)", got, compress, want)
	}
}

func TestCopySavedPathFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmp := t.TempDir()

	labDir := filepath.Join(tmp, "clab-lab", "node1")
	srcPath := filepath.Join(labDir, "config", "config.cfg")
	if err := os.MkdirAll(filepath.Dir(srcPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dstNodeDir := filepath.Join(tmp, "dst", "node1")
	ts := "240101_010101"

	if err := copySavedPath(ctx, srcPath, labDir, dstNodeDir, ts); err != nil {
		t.Fatalf("copySavedPath: %v", err)
	}

	latest := filepath.Join(dstNodeDir, "config", "config.cfg")
	got, err := os.ReadFile(latest)
	if err != nil {
		t.Fatalf("read latest: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("latest content = %q, want %q", string(got), "hello")
	}

	tsPath := filepath.Join(dstNodeDir, "config", "config-240101_010101.cfg.gz")
	if err := assertGzipContains(tsPath, "hello"); err != nil {
		t.Fatalf("timestamped gzip: %v", err)
	}
}

func TestCopySavedPathDir(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmp := t.TempDir()

	labDir := filepath.Join(tmp, "clab-lab", "node1")
	srcDir := filepath.Join(labDir, "xr-storage")
	srcFile := filepath.Join(srcDir, "config.txt")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("config"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dstNodeDir := filepath.Join(tmp, "dst", "node1")
	ts := "240101_010101"

	if err := copySavedPath(ctx, srcDir, labDir, dstNodeDir, ts); err != nil {
		t.Fatalf("copySavedPath: %v", err)
	}

	latest := filepath.Join(dstNodeDir, "xr-storage", "config.txt")
	got, err := os.ReadFile(latest)
	if err != nil {
		t.Fatalf("read latest: %v", err)
	}
	if string(got) != "config" {
		t.Fatalf("latest content = %q, want %q", string(got), "config")
	}

	archive := filepath.Join(dstNodeDir, "xr-storage-240101_010101.tar.gz")
	if err := assertTarGzContains(archive, filepath.ToSlash(filepath.Join("xr-storage", "config.txt"))); err != nil {
		t.Fatalf("archive check: %v", err)
	}
}

func assertGzipContains(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	b, err := io.ReadAll(gr)
	if err != nil {
		return err
	}

	if string(b) != want {
		return fmt.Errorf("gzip content = %q, want %q", string(b), want)
	}

	return nil
}

func assertTarGzContains(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name == want {
			return nil
		}
	}

	return fmt.Errorf("archive %q missing %q", path, want)
}
