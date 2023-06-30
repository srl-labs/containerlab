package utils

import (
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

func TestSetDelayJitterLoss(t *testing.T) {
	type args struct {
		nsFd   int
		link   netlink.Link
		delay  time.Duration
		jitter time.Duration
		loss   float64
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "no link given",
			args: args{
				link: nil,
			},
			wantErr: true,
		},
		{
			name: "parameters uninitialized", // will only raise log warning
			args: args{
				link: &netlink.Dummy{
					LinkAttrs: netlink.LinkAttrs{
						Name: "dummy",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "jitter without delay set",
			args: args{
				link: &netlink.Dummy{
					LinkAttrs: netlink.LinkAttrs{
						Name: "dummy",
					},
				},
				jitter: time.Millisecond * 2,
			},
			wantErr: true,
		},
		{
			name: "loss > 100",
			args: args{
				link: &netlink.Dummy{
					LinkAttrs: netlink.LinkAttrs{
						Name: "dummy",
					},
				},
				loss: 101.0,
			},
			wantErr: true,
		},
		{
			name: "loss < 0",
			args: args{
				link: &netlink.Dummy{
					LinkAttrs: netlink.LinkAttrs{
						Name: "dummy",
					},
				},
				loss: -1.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetDelayJitterLoss(tt.args.nsFd, tt.args.link, tt.args.delay, tt.args.jitter, tt.args.loss); (err != nil) != tt.wantErr {
				t.Errorf("SetDelayJitterLoss() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
