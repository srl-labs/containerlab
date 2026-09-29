package types

import (
	"reflect"
	"testing"
)

func TestSplitDockerAndTailscalePorts(t *testing.T) {
	docker, ts, err := SplitDockerAndTailscalePorts([]string{
		"80:8080",
		"8022:22/ts",
		"8574:57400/TS",
		"9000-9001:80-81/ts",
		"55555:43555/udp",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDocker := []string{"80:8080", "55555:43555/udp"}
	if !reflect.DeepEqual(docker, wantDocker) {
		t.Fatalf("docker = %#v, want %#v", docker, wantDocker)
	}
	wantTS := []TailscalePort{
		{Listen: 8022, Dest: 22},
		{Listen: 8574, Dest: 57400},
		{Listen: 9000, Dest: 80},
		{Listen: 9001, Dest: 81},
	}
	if !reflect.DeepEqual(ts, wantTS) {
		t.Fatalf("ts = %#v, want %#v", ts, wantTS)
	}
}

func TestSplitDockerAndTailscalePortsInvalid(t *testing.T) {
	for _, spec := range []string{
		"not-a-port/ts",
		"22/ts",
		"0:22/ts",
		"8022:0/ts",
		"127.0.0.1:8022:22/ts",
		"8000-8010:22/ts",
	} {
		if _, _, err := SplitDockerAndTailscalePorts([]string{spec}); err == nil {
			t.Errorf("%q: expected error", spec)
		}
	}
}

func TestTailscaleConfigProxy(t *testing.T) {
	if (&TailscaleConfig{AuthMode: "sso"}).Proxy() != true {
		t.Fatal("sso should be proxy")
	}
	if (&TailscaleConfig{AuthMode: "SSO"}).Proxy() != true {
		t.Fatal("SSO should be proxy")
	}
	if (&TailscaleConfig{AuthKey: "tskey"}).Proxy() {
		t.Fatal("auth-key only is not proxy")
	}
	var n *TailscaleConfig
	if n.Proxy() {
		t.Fatal("nil config is not proxy")
	}
}
