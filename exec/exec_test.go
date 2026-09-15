package exec

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
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

func TestExecCollectionLogPreservesAddOrder(t *testing.T) {
	collection := NewExecCollection()
	collection.Add("h1", NewExecResult(NewExecCmdFromSlice([]string{"echo", "h1"})))
	collection.Add("h2", NewExecResult(NewExecCmdFromSlice([]string{"echo", "h2"})))

	var output bytes.Buffer
	log.SetOutput(&output)
	defer log.SetOutput(os.Stderr)

	collection.Log()

	logOutput := output.String()
	h1Index := strings.Index(logOutput, "node=h1")
	h2Index := strings.Index(logOutput, "node=h2")
	if h1Index == -1 || h2Index == -1 {
		t.Fatalf("expected both command results in log output, got:\n%s", logOutput)
	}
	if h1Index > h2Index {
		t.Fatalf("log output did not preserve add order:\n%s", logOutput)
	}
}
