package utils

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
)

func TestExtractDNSServersFromResolvConf(t *testing.T) {
	type args struct {
		filesys   fs.FS
		filenames []string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "One file local dns empty result",
			args: args{
				filesys: fstest.MapFS{
					"etc/resolv.conf": &fstest.MapFile{
						Data: []byte(
							`
# This is /run/systemd/resolve/stub-resolv.conf managed by man:systemd-resolved(8).
# Do not edit.
#
# This file might be symlinked as /etc/resolv.conf. If you're looking at
# /etc/resolv.conf and seeing this text, you have followed the symlink.

nameserver 127.0.0.53
options edns0 trust-ad
search .					
`,
						),
					},
				},
				filenames: []string{"etc/resolv.conf"},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "One file with more than 3 dns servers",
			args: args{
				filesys: fstest.MapFS{
					"etc/resolv.conf": &fstest.MapFile{
						Data: []byte(
							`
# This is /run/systemd/resolve/stub-resolv.conf managed by man:systemd-resolved(8).
# Do not edit.
#
# This file might be symlinked as /etc/resolv.conf. If you're looking at
# /etc/resolv.conf and seeing this text, you have followed the symlink.

nameserver 1.1.1.1
nameserver 1.1.1.2
nameserver 1.1.1.3
nameserver 1.1.1.4
nameserver 1.1.1.5
options edns0 trust-ad
search .					
`,
						),
					},
				},
				filenames: []string{"etc/resolv.conf"},
			},
			want:    []string{"1.1.1.1", "1.1.1.2", "1.1.1.3"},
			wantErr: false,
		},
		{
			name: "Two files local dns and two remote, two results",
			args: args{
				filesys: fstest.MapFS{
					"etc/resolv.conf": &fstest.MapFile{
						Data: []byte(
							`
# This is /run/systemd/resolve/stub-resolv.conf managed by man:systemd-resolved(8).
# Do not edit.
#
# This file might be symlinked as /etc/resolv.conf. If you're looking at
# /etc/resolv.conf and seeing this text, you have followed the symlink.

nameserver 1.1.1.1
options edns0 trust-ad
search .					
`,
						),
					},
					"etc/someother/resolv.conf": &fstest.MapFile{
						Data: []byte(
							`
# This is /run/systemd/resolve/stub-resolv.conf managed by man:systemd-resolved(8).
# Do not edit.

nameserver 127.0.0.53
nameserver 8.8.8.8
options edns0 trust-ad
search .					
`,
						),
					},
				},
				filenames: []string{"etc/resolv.conf", "etc/someother/resolv.conf"},
			},
			want:    []string{"1.1.1.1", "8.8.8.8"},
			wantErr: false,
		},
		{
			name: "Duplicate 8.8.8.8",
			args: args{
				filesys: fstest.MapFS{
					"etc/resolv.conf": &fstest.MapFile{
						Data: []byte(
							`
# Do not edit.
nameserver 1.1.1.1
nameserver 8.8.8.8

options edns0 trust-ad
search .					
`,
						),
					},
					"etc/someother/resolv.conf": &fstest.MapFile{
						Data: []byte(
							`
nameserver 2.2.2.2
nameserver 8.8.8.8
options edns0 trust-ad
search .					
`,
						),
					},
				},
				filenames: []string{"etc/resolv.conf", "etc/someother/resolv.conf"},
			},
			want:    []string{"1.1.1.1", "2.2.2.2", "8.8.8.8"},
			wantErr: false,
		},
		{
			name: "Files do not exist",
			args: args{
				filesys:   fstest.MapFS{},
				filenames: []string{"etc/resolv.conf", "etc/someother/resolv.conf"},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractDNSServersFromResolvConf(tt.args.filesys, tt.args.filenames)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractDNSServerFromResolvConf() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Error(diff)
			}
		})
	}
}
