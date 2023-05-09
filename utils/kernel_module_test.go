package utils

import "testing"

func TestIsKernelModuleLoaded(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "one",
			args: args{
				name: "ip_tables",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "one",
			args: args{
				name: "someXRaND0mStr1nG!",
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsKernelModuleLoaded(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsKernelModuleLoaded() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsKernelModuleLoaded() = %v, want %v", got, tt.want)
			}
		})
	}
}
