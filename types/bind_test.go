package types

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewBind(t *testing.T) {
	tests := []struct {
		name string
		src  string
		dst  string
		mode string
		want *Bind
	}{
		{
			name: "all_fields",
			src:  "/host/path",
			dst:  "/container/path",
			mode: "ro",
			want: &Bind{
				src:  "/host/path",
				dst:  "/container/path",
				mode: "ro",
			},
		},
		{
			name: "empty_mode",
			src:  "/host/path",
			dst:  "/container/path",
			mode: "",
			want: &Bind{
				src:  "/host/path",
				dst:  "/container/path",
				mode: "",
			},
		},
		{
			name: "empty_src",
			src:  "",
			dst:  "/container/path",
			mode: "rw",
			want: &Bind{
				src:  "",
				dst:  "/container/path",
				mode: "rw",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBind(tt.src, tt.dst, tt.mode)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(Bind{})); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewBindFromString(t *testing.T) {
	tests := []struct {
		name    string
		bind    string
		want    *Bind
		wantErr bool
		errStr  string
	}{
		{
			name: "full_bind",
			bind: "/host/path:/container/path:ro",
			want: &Bind{
				src:  "/host/path",
				dst:  "/container/path",
				mode: "ro",
			},
			wantErr: false,
		},
		{
			name: "no_mode",
			bind: "/host/path:/container/path",
			want: &Bind{
				src:  "/host/path",
				dst:  "/container/path",
				mode: "",
			},
			wantErr: false,
		},
		{
			name: "volume_only",
			bind: "/container/path",
			want: &Bind{
				src:  "",
				dst:  "/container/path",
				mode: "",
			},
			wantErr: false,
		},
		{
			name: "empty_string",
			bind: "",
			want: &Bind{
				src:  "",
				dst:  "",
				mode: "",
			},
			wantErr: false,
		},
		{
			name:    "too_many_colons",
			bind:    "/host:/container:ro:extra",
			want:    nil,
			wantErr: true,
			errStr:  "unable to parse bind",
		},
		{
			name: "with_spaces",
			bind: "/host path:/container path:rw",
			want: &Bind{
				src:  "/host path",
				dst:  "/container path",
				mode: "rw",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewBindFromString(tt.bind)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewBindFromString() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
					t.Fatalf("expected error containing %q, got %q", tt.errStr, err.Error())
				}

				return
			}

			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(Bind{})); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBind_Accessors(t *testing.T) {
	tests := []struct {
		name     string
		bind     *Bind
		wantSrc  string
		wantDst  string
		wantMode string
	}{
		{
			name: "all_fields",
			bind: &Bind{
				src:  "/host/path",
				dst:  "/container/path",
				mode: "ro",
			},
			wantSrc:  "/host/path",
			wantDst:  "/container/path",
			wantMode: "ro",
		},
		{
			name: "empty_fields",
			bind: &Bind{
				src:  "",
				dst:  "",
				mode: "",
			},
			wantSrc:  "",
			wantDst:  "",
			wantMode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bind.Src(); got != tt.wantSrc {
				t.Errorf("Bind.Src() = %v, want %v", got, tt.wantSrc)
			}

			if got := tt.bind.Dst(); got != tt.wantDst {
				t.Errorf("Bind.Dst() = %v, want %v", got, tt.wantDst)
			}

			if got := tt.bind.Mode(); got != tt.wantMode {
				t.Errorf("Bind.Mode() = %v, want %v", got, tt.wantMode)
			}
		})
	}
}

func TestBind_String(t *testing.T) {
	tests := []struct {
		name string
		bind *Bind
		want string
	}{
		{
			name: "full_bind",
			bind: &Bind{
				src:  "/host/path",
				dst:  "/container/path",
				mode: "ro",
			},
			want: "/host/path:/container/path:ro",
		},
		{
			name: "no_mode",
			bind: &Bind{
				src:  "/host/path",
				dst:  "/container/path",
				mode: "",
			},
			want: "/host/path:/container/path",
		},
		{
			name: "volume_only",
			bind: &Bind{
				src:  "",
				dst:  "/container/path",
				mode: "",
			},
			want: "/container/path",
		},
		{
			name: "volume_with_mode",
			bind: &Bind{
				src:  "",
				dst:  "/container/path",
				mode: "rw",
			},
			want: "/container/path:rw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bind.String()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBinds_ToStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		binds Binds
		want  []string
	}{
		{
			name:  "empty_binds",
			binds: Binds{},
			want:  []string{},
		},
		{
			name: "single_bind",
			binds: Binds{
				&Bind{src: "/host", dst: "/container", mode: "ro"},
			},
			want: []string{"/host:/container:ro"},
		},
		{
			name: "multiple_binds",
			binds: Binds{
				&Bind{src: "/host1", dst: "/container1", mode: "ro"},
				&Bind{src: "/host2", dst: "/container2", mode: "rw"},
				&Bind{src: "", dst: "/volume", mode: ""},
			},
			want: []string{
				"/host1:/container1:ro",
				"/host2:/container2:rw",
				"/volume",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.binds.ToStringSlice()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
