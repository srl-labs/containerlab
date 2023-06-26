package utils

import "testing"

func Test_pidFromNSPath(t *testing.T) {
	type args struct {
		ns string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "Test One",
			args: args{
				ns: "/proc/6845/ns/net",
			},
			want: 6845,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pidFromNSPath(tt.args.ns); got != tt.want {
				t.Errorf("pidFromNSPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
