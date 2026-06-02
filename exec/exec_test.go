package exec

import (
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
)

func TestParseExecOutputFormat(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "Valid value: plain",
			want:    clabconstants.FormatPlain,
			wantErr: false,
			args: args{
				s: "plain",
			},
		},
		{
			name:    "Valid value: pLAiN",
			want:    clabconstants.FormatPlain,
			wantErr: false,
			args: args{
				s: "plain",
			},
		},
		{
			name:    "Valid value: json",
			want:    clabconstants.FormatJSON,
			wantErr: false,
			args: args{
				s: clabconstants.FormatJSON,
			},
		},
		{
			name:    "Valid value: table (mapped to plain)",
			want:    clabconstants.FormatPlain,
			wantErr: false,
			args: args{
				s: "table",
			},
		},
		{
			name:    "Invalid value: foobar",
			want:    "",
			wantErr: true,
			args: args{
				s: "foobar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseExecOutputFormat(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExecOutputFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseExecOutputFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}
