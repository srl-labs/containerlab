package cmd

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

func TestParseCaptureTargets(t *testing.T) {
	t.Parallel()

	targets, err := parseCaptureTargets([]string{
		"srl1:e1-1,ethernet-1/2", " sros : 1/1/3 ", "r1:eth1",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []clablabruntime.CaptureTarget{
		{Node: "srl1", Interface: "e1-1"},
		{Node: "srl1", Interface: "ethernet-1/2"},
		{Node: "sros", Interface: "1/1/3"},
		{Node: "r1", Interface: "eth1"},
	}
	if !slices.Equal(targets, want) {
		t.Fatalf("targets = %+v, want %+v", targets, want)
	}

	for _, bad := range [][]string{nil, {"srl1"}, {":e1-1"}, {"srl1:"}, {"srl1:e1-1,,e1-2"}} {
		if _, err := parseCaptureTargets(bad); err == nil {
			t.Errorf("parseCaptureTargets(%q) succeeded", bad)
		}
	}
}

func TestCaptureCommandFlags(t *testing.T) {
	o := GetOptions()
	cmd, err := captureCmd(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.ParseFlags([]string{
		"--target", "srl1:e1-1", "--target", "srl2:e1-1,e1-2", "--duration", "90m",
		"--packet-limit", "100", "--snap-length", "128", "-w", "out.pcapng",
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(o.Capture.Targets, []string{"srl1:e1-1", "srl2:e1-1,e1-2"}) ||
		o.Capture.Duration.Minutes() != 90 || o.Capture.PacketLimit != 100 ||
		o.Capture.SnapLength != 128 || o.Capture.Write != "out.pcapng" {
		t.Fatalf("capture options = %+v", o.Capture)
	}
	if def := cmd.Flag("write").DefValue; def != "-" {
		t.Fatalf("--write default = %q, want stdout", def)
	}
}

func TestPrintCaptureInterfaces(t *testing.T) {
	t.Parallel()

	interfaces := []clablabruntime.CaptureInterface{
		{Node: "srl1", Interface: "e1-1", Alias: "ethernet-1/1", PeerNode: "srl2", PeerInterface: "ethernet-1/1"},
		{Node: "srl1", Interface: "e1-2", PeerNode: "host", PeerInterface: "srl1-e1-2"},
	}

	var out bytes.Buffer
	if err := printCaptureInterfaces(&out, interfaces, "json"); err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded[0]["peer_node"] != "srl2" || decoded[0]["alias"] != "ethernet-1/1" {
		t.Fatalf("json = %s", out.String())
	}
	if _, hasAlias := decoded[1]["alias"]; hasAlias {
		t.Fatalf("an empty alias is printed: %s", out.String())
	}

	out.Reset()
	if err := printCaptureInterfaces(&out, nil, "json"); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("empty json = %q, %v", out.String(), err)
	}

	out.Reset()
	if err := printCaptureInterfaces(&out, interfaces, "table"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Peer Interface", "ethernet-1/1", "srl1-e1-2", "N/A"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("table does not contain %q:\n%s", want, out.String())
		}
	}
}
