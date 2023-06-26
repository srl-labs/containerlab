package utils

import (
	"testing"
)

func TestSetDelay(t *testing.T) {
	type args struct {
		nsPath string
		iface  string
		delay  int64
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "one",
			args: args{
				iface:  "eth1",
				delay:  500,
				nsPath: "/proc/220224/ns/net",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetDelay(tt.args.nsPath, tt.args.iface, tt.args.delay); (err != nil) != tt.wantErr {
				t.Errorf("SetDelay() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
