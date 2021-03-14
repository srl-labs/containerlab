package clab

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCheckSockType(t *testing.T) {
	tests := map[string]struct {
		got  string
		want error
	}{
		"correct-type": {
			got:  "tcp",
			want: nil,
		},
		"incorrect-type": {
			got:  "dns",
			want: fmt.Errorf("mysocketio type dns is not supported. Supported types are tcp/tls/http/https"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := checkSockType(tc.got)

			if err != nil && tc.want != nil && (err.Error() != tc.want.Error()) {
				t.Fatalf("wanted '%v' got '%v'", tc.want, err)
			}
			if err != nil && tc.want == nil {
				t.Fatalf("wanted '%v' got '%v'", tc.want, err)
			}

		})
	}
}

func TestCheckSockPort(t *testing.T) {
	tests := map[string]struct {
		got  int
		want error
	}{
		"correct-port": {
			got:  22,
			want: nil,
		},
		"negative-port": {
			got:  -22,
			want: fmt.Errorf("incorrect port number -22"),
		},
		"large-port": {
			got:  123123123,
			want: fmt.Errorf("incorrect port number 123123123"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := checkSockPort(tc.got)

			if err != nil && tc.want != nil && (err.Error() != tc.want.Error()) {
				t.Fatalf("wanted '%v' got '%v'", tc.want, err)
			}
			if err != nil && tc.want == nil {
				t.Fatalf("wanted '%v' got '%v'", tc.want, err)
			}

		})
	}
}

func TestParseSocketCfg(t *testing.T) {
	tests := map[string]struct {
		got  string
		want mysocket
		err  error
	}{
		"simple-tcp": {
			got: "tcp/22",
			want: mysocket{
				Stype: "tcp",
				Port:  22,
			},
			err: nil,
		},
		"simple-http": {
			got: "http/8080",
			want: mysocket{
				Stype: "http",
				Port:  8080,
			},
			err: nil,
		},
		"wrong-type": {
			got:  "stcp/22",
			want: mysocket{},
			err:  fmt.Errorf(""),
		},
		"with-email-and-domain": {
			got: "tls/22/a@b.com,c.com",
			want: mysocket{
				Stype:          "tls",
				Port:           22,
				AllowedDomains: []string{"c.com"},
				AllowedEmails:  []string{"a@b.com"},
			},
			err: nil,
		},
		"wrong-num-of-sections": {
			got:  "tcp/22/a@b.com/test",
			want: mysocket{},
			err:  fmt.Errorf(""),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseSocketCfg(tc.got)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("parseSocketCfg() mismatch (-want +got):\n%s", diff)
			}

			switch tc.err {
			case nil:
				if err != nil {
					t.Errorf("unexpected error %v", err)
				}
			case fmt.Errorf(""):
				if err != nil {
					t.Errorf("expected to have an errorm but got nil instead")
				}

			}

		})
	}
}

func TestParseAllowedUsers(t *testing.T) {
	tests := map[string]struct {
		got  string
		want struct {
			Domains []string
			Emails  []string
		}
	}{
		"single-email": {
			got: "a@b.com",
			want: struct {
				Domains []string
				Emails  []string
			}{
				Domains: nil,
				Emails:  []string{"a@b.com"},
			},
		},
		"two-emails": {
			got: "a@b.com,x@y.com",
			want: struct {
				Domains []string
				Emails  []string
			}{
				Domains: nil,
				Emails:  []string{"a@b.com", "x@y.com"},
			},
		},
		"two-emails-with-spaces": {
			got: " a@b.com , x@y.com",
			want: struct {
				Domains []string
				Emails  []string
			}{
				Domains: nil,
				Emails:  []string{"a@b.com", "x@y.com"},
			},
		},
		"email-and-domain": {
			got: "a@b.com,dom.com",
			want: struct {
				Domains []string
				Emails  []string
			}{
				Domains: []string{"dom.com"},
				Emails:  []string{"a@b.com"},
			},
		},
		"many-emails-many-domains": {
			got: "a@b.com,dom.com,x@y.com,abc.net",
			want: struct {
				Domains []string
				Emails  []string
			}{
				Domains: []string{"dom.com", "abc.net"},
				Emails:  []string{"a@b.com", "x@y.com"},
			},
		},
		"empty-value": {
			got: "a@b.com,,x@y.com,",
			want: struct {
				Domains []string
				Emails  []string
			}{
				Domains: nil,
				Emails:  []string{"a@b.com", "x@y.com"},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var got struct {
				Domains []string
				Emails  []string
			}
			got.Domains, got.Emails, _ = parseAllowedUsers(tc.got)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("parseSocketCfg() mismatch (-want +got):\n%s", diff)
			}

		})
	}
}

func TestCreateSockCmd(t *testing.T) {
	tests := map[string]struct {
		got struct {
			MS   mysocket
			Name string
		}
		want string
	}{
		"single-email": {
			got: struct {
				MS   mysocket
				Name string
			}{
				MS: mysocket{
					Stype:         "tls",
					Port:          22,
					AllowedEmails: []string{"x@y.com"},
				},
				Name: "test",
			},
			want: "mysocketctl socket create -t tls -n clab-test-tls-22 -c -e 'x@y.com'",
		},
		"single-domain": {
			got: struct {
				MS   mysocket
				Name string
			}{
				MS: mysocket{
					Stype:          "tls",
					Port:           22,
					AllowedDomains: []string{"y.com"},
				},
				Name: "test",
			},
			want: "mysocketctl socket create -t tls -n clab-test-tls-22 -c -d 'y.com'",
		},
		"domains-and-emails": {
			got: struct {
				MS   mysocket
				Name string
			}{
				MS: mysocket{
					Stype:          "tls",
					Port:           22,
					AllowedDomains: []string{"y.com"},
					AllowedEmails:  []string{"x@z.com"},
				},
				Name: "test",
			},
			want: "mysocketctl socket create -t tls -n clab-test-tls-22 -c -d 'y.com' -e 'x@z.com'",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := createSockCmd(tc.got.MS, tc.got.Name)

			if diff := cmp.Diff(tc.want, cmd); diff != "" {
				t.Errorf("parseSocketCfg() mismatch (-want +got):\n%s", diff)
			}

		})
	}
}
