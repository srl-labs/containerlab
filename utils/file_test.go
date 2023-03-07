// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import "testing"

func Test_CalcFilename(t *testing.T) {
	type args struct {
		rawUrl string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "regular filename url",
			args: args{
				rawUrl: "http://myserver.foo/download/raw/node1.cfg",
			},
			want: "node1.cfg",
		},
		{
			name: "folder URL",
			args: args{
				rawUrl: "http://myserver.foo/download/raw/",
			},
			want: CalcFilename_Undefined,
		},
		{
			name: "with get parameters",
			args: args{
				rawUrl: "http://myserver.foo/download/raw/node1.cfg?foo=bar&bar=foo",
			},
			want: "node1.cfg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalcFilename(tt.args.rawUrl); got != tt.want {
				t.Errorf("calcFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}
