package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

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

func TestParseKernelVersion(t *testing.T) {
	tests := []struct {
		input     []byte
		expectErr bool
		expected  *KernelVersion
	}{
		{[]byte("123.45.6789 example"), false, &KernelVersion{
			Major: 123, Minor: 45,
			Revision: 6789, Remainder: " example",
		}},
		{[]byte("1.2.3-abc"), false, &KernelVersion{Major: 1, Minor: 2, Revision: 3, Remainder: "-abc"}},
		{[]byte("invalid"), true, nil},
		{[]byte(""), true, nil},
	}

	for _, test := range tests {
		result, err := parseKernelVersion(test.input)

		if (err != nil) != test.expectErr {
			t.Errorf("Error expectation mismatch. Input: %s, Expected Error: %v, Actual Error: %v", test.input, test.expectErr, err)
		}

		if d := cmp.Diff(result, test.expected); d != "" {
			t.Errorf("parseKernelVersion got = %+v, want %+v; Diff: %s", result, test.expected, d)
		}
	}
}

func TestKernelVersionGreaterOrEqual(t *testing.T) {
	tests := []struct {
		version      *KernelVersion
		compare      *KernelVersion
		expectResult bool
	}{
		{&KernelVersion{Major: 1, Minor: 2, Revision: 3}, &KernelVersion{Major: 1, Minor: 2, Revision: 3}, true},
		{&KernelVersion{Major: 1, Minor: 2, Revision: 3}, &KernelVersion{Major: 1, Minor: 2, Revision: 2}, true},
		{&KernelVersion{Major: 1, Minor: 2, Revision: 3}, &KernelVersion{Major: 1, Minor: 3, Revision: 3}, false},
		{&KernelVersion{Major: 2, Minor: 3, Revision: 4}, &KernelVersion{Major: 1, Minor: 2, Revision: 3}, true},
		{&KernelVersion{Major: 2, Minor: 1, Revision: 1}, &KernelVersion{Major: 1, Minor: 2, Revision: 3}, true},
		{&KernelVersion{Major: 2, Minor: 3, Revision: 1}, &KernelVersion{Major: 2, Minor: 2, Revision: 3}, true},
		{&KernelVersion{Major: 2, Minor: 3, Revision: 4}, &KernelVersion{Major: 3, Minor: 4, Revision: 5}, false},
		{&KernelVersion{Major: 2, Minor: 3, Revision: 4}, &KernelVersion{Major: 2, Minor: 3, Revision: 5}, false},
	}

	for _, test := range tests {
		result := test.version.GreaterOrEqual(test.compare)

		if result != test.expectResult {
			t.Errorf("Result mismatch. Version: %+v, Compare: %+v, Expected: %v, Actual: %v",
				test.version, test.compare, test.expectResult, result)
		}
	}
}
