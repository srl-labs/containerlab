package utils

// import (
// 	"testing"
// 	"time"
// )

// func TestSetDelay(t *testing.T) {
// 	type args struct {
// 		pid int
// 		iface  string
// 		delay  int // in ms
// 		jitter int // in ms
// 		loss   uint
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr bool
// 	}{
// 		{
// 			name: "one",
// 			args: args{
// 				iface:  "eth1",
// 				delay:  100,
// 				jitter: 50,
// 				loss:   5,
// 				pid: 302801,
// 			},
// 			wantErr: false,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {

// 			latency := time.Millisecond * time.Duration(tt.args.delay)
// 			jitter := time.Millisecond * time.Duration(tt.args.jitter)

// 			if err := SetDelayJitterLoss(tt.args.pid, tt.args.iface, &latency, &jitter, &tt.args.loss); (err != nil) != tt.wantErr {
// 				t.Errorf("SetDelay() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }
