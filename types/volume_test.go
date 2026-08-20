package types

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewVolumeFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		volume  string
		want    *Volume
		wantErr bool
		errStr  string
	}{
		{
			name:   "full_volume",
			volume: "namedvol:/container/path:ro,nocopy",
			want: &Volume{
				src:  "namedvol",
				dst:  "/container/path",
				mode: "ro,nocopy",
			},
			wantErr: false,
		},
		{
			name:   "no_mode",
			volume: "namedvol:/container/path",
			want: &Volume{
				src:  "namedvol",
				dst:  "/container/path",
				mode: "",
			},
			wantErr: false,
		},
		{
			name:   "anonymous_volume",
			volume: "/container/path",
			want: &Volume{
				src:  "",
				dst:  "/container/path",
				mode: "",
			},
			wantErr: false,
		},
		{
			name:   "trailing_colon_empty_mode",
			volume: "namedvol:/container/path:",
			want: &Volume{
				src:  "namedvol",
				dst:  "/container/path",
				mode: "",
			},
			wantErr: false,
		},
		{
			name:    "host_path_error",
			volume:  "/host/path:/container/path",
			wantErr: true,
			errStr:  "references a host path; please use the binds stanza instead",
		},
		{
			name:    "empty_string",
			volume:  "",
			wantErr: true,
			errStr:  "unable to parse volume",
		},
		{
			name:    "too_many_colons",
			volume:  "namedvol:/container:ro:extra",
			wantErr: true,
			errStr:  "unable to parse volume",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewVolumeFromString(tt.volume)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewVolumeFromString() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
					t.Fatalf("expected error containing %q, got %q", tt.errStr, err.Error())
				}

				return
			}

			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(Volume{})); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestVolume_Accessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		volume   *Volume
		wantSrc  string
		wantDst  string
		wantMode string
	}{
		{
			name: "all_fields",
			volume: &Volume{
				src:  "namedvol",
				dst:  "/container/path",
				mode: "ro",
			},
			wantSrc:  "namedvol",
			wantDst:  "/container/path",
			wantMode: "ro",
		},
		{
			name: "empty_fields",
			volume: &Volume{
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.volume.Src(); got != tt.wantSrc {
				t.Errorf("Volume.Src() = %v, want %v", got, tt.wantSrc)
			}

			if got := tt.volume.Dst(); got != tt.wantDst {
				t.Errorf("Volume.Dst() = %v, want %v", got, tt.wantDst)
			}

			if got := tt.volume.Mode(); got != tt.wantMode {
				t.Errorf("Volume.Mode() = %v, want %v", got, tt.wantMode)
			}
		})
	}
}

func TestVolume_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		volume *Volume
		want   string
	}{
		{
			name: "full_volume",
			volume: &Volume{
				src:  "namedvol",
				dst:  "/container/path",
				mode: "ro,nocopy",
			},
			want: "namedvol:/container/path:ro,nocopy",
		},
		{
			name: "no_mode",
			volume: &Volume{
				src:  "namedvol",
				dst:  "/container/path",
				mode: "",
			},
			want: "namedvol:/container/path",
		},
		{
			name: "anonymous",
			volume: &Volume{
				src:  "",
				dst:  "/container/path",
				mode: "",
			},
			want: "/container/path",
		},
		{
			name: "anonymous_with_mode",
			volume: &Volume{
				src:  "",
				dst:  "/container/path",
				mode: "ro",
			},
			want: "/container/path:ro",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.volume.String()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestVolume_Options(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		volume *Volume
		want   []string
	}{
		{
			name: "no_mode",
			volume: &Volume{
				mode: "",
			},
			want: nil,
		},
		{
			name: "single_option",
			volume: &Volume{
				mode: "ro",
			},
			want: []string{"ro"},
		},
		{
			name: "multiple_options",
			volume: &Volume{
				mode: "ro,nocopy",
			},
			want: []string{"ro", "nocopy"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tt.want, tt.volume.Options()); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseVolumeOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []string
		want VolumeOptions
	}{
		{
			name: "read_only_with_nocopy",
			opts: []string{"ro", "nocopy"},
			want: VolumeOptions{
				ReadOnly: true,
				NoCopy:   true,
				Unknown:  nil,
			},
		},
		{
			name: "ignore_rw_and_empty",
			opts: []string{"rw", "", "ro"},
			want: VolumeOptions{
				ReadOnly: true,
				NoCopy:   false,
				Unknown:  nil,
			},
		},
		{
			name: "unknown_options",
			opts: []string{"volume-nocopy", "z", "user:1000"},
			want: VolumeOptions{
				ReadOnly: false,
				NoCopy:   true,
				Unknown:  []string{"z", "user:1000"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ParseVolumeOptions(tt.opts)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
