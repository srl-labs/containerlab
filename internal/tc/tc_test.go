package tc

import (
	"net"
	"testing"
	"time"
)

func TestSetDelayJitterLoss(t *testing.T) {
	type args struct {
		nodeName string
		nsFd     int
		link     *net.Interface
		delay    time.Duration
		jitter   time.Duration
		loss     float64
		rate     uint64
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
				link: &net.Interface{
					Name: "dummy",
				},
			},
			wantErr: false,
		},
		{
			name: "jitter without delay set",
			args: args{
				link: &net.Interface{
					Name: "dummy",
				},
				jitter: time.Millisecond * 2,
			},
			wantErr: true,
		},
		{
			name: "loss > 100",
			args: args{
				link: &net.Interface{
					Name: "dummy",
				},
				loss: 101.0,
			},
			wantErr: true,
		},
		{
			name: "loss < 0",
			args: args{
				link: &net.Interface{
					Name: "dummy",
				},
				loss: -1.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetImpairments(tt.args.nodeName, tt.args.nsFd, tt.args.link, tt.args.delay, tt.args.jitter, tt.args.loss, tt.args.rate); (err != nil) != tt.wantErr {
				t.Errorf("SetDelayJitterLoss() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
