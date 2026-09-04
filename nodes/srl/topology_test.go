package srl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
)

// modularVariants is the catalog of modular chassis flavors containerlab ships, transcribed
// from the SR Linux platform tables. It is the reference the embedded files are checked against.
// The chassis, cpm and imm names are the ones a booted node reports for the numeric ids next to
// them, captured from `info from state platform`.
var modularVariants = map[string]struct {
	chassisType int
	cpmCardType int
	cardType    int
	mdaType     int
	chassisName string
	cpm         string
	imm         string
	chassisMode string
}{
	"ixr-6e-gen2cp-qsfpdd": {
		68, 184, 182, 199,
		"7250 IXR-6e-gen2cp", "cpm4-ixr", "imm36-400g-qsfpdd", "GEN2CP_ONLY",
	},
	"ixr-6e-gen2cp-qsfp28": {
		68, 184, 183, 219,
		"7250 IXR-6e-gen2cp", "cpm4-ixr", "imm60-100g-qsfp28", "GEN2CP_ONLY",
	},
	"ixr-6e-gen2cp-sync": {
		68, 41, 43, 199,
		"7250 IXR-6e-gen2cp", "cpm4-t-sync-ixr", "imm2-36-400g-sync-qsfpdd", "GEN2CP_ONLY",
	},
	"ixr-6e-gen3-qsfpdd": {
		68, 184, 52, 11,
		"7250 IXR-6e-gen3", "cpm4-ixr", "imm3-36-800g-qsfpdd", "GEN3_ONLY",
	},
	"ixr-6e-gen3-osfp": {
		68, 184, 53, 14,
		"7250 IXR-6e-gen3", "cpm4-ixr", "imm3-36-800g-osfp", "GEN3_ONLY",
	},
	"ixr-10e-gen2cp-qsfpdd": {
		69, 184, 182, 199,
		"7250 IXR-10e-gen2cp", "cpm4-ixr", "imm36-400g-qsfpdd", "GEN2CP_ONLY",
	},
	"ixr-10e-gen2cp-qsfp28": {
		69, 184, 183, 219,
		"7250 IXR-10e-gen2cp", "cpm4-ixr", "imm60-100g-qsfp28", "GEN2CP_ONLY",
	},
	"ixr-10e-gen2cp-sync": {
		69, 41, 43, 199,
		"7250 IXR-10e-gen2cp", "cpm4-t-sync-ixr", "imm2-36-400g-sync-qsfpdd", "GEN2CP_ONLY",
	},
	"ixr-10e-gen3-qsfpdd": {
		69, 184, 52, 11,
		"7250 IXR-10e-gen3", "cpm4-ixr", "imm3-36-800g-qsfpdd", "GEN3_ONLY",
	},
	"ixr-10e-gen3-osfp": {
		69, 184, 53, 14,
		"7250 IXR-10e-gen3", "cpm4-ixr", "imm3-36-800g-osfp", "GEN3_ONLY",
	},
	"ixr-18e-qsfpdd": {
		70, 184, 33, 13,
		"7250 IXR-18e-gen3", "cpm4-ixr", "imm3-18-800g-qsfpdd", "GEN3_ONLY",
	},
	"ixr-18e-gen3-sync": {
		70, 25, 46, 11,
		"7250 IXR-18e-gen3", "cpm5-t-ixr", "imm3-36-800g-sync-qsfpdd", "GEN3_ONLY",
	},
	"ixr-18e-gen3-osfp": {
		70, 184, 53, 14,
		"7250 IXR-18e-gen3", "cpm4-ixr", "imm3-36-800g-osfp", "GEN3_ONLY",
	},
}

func TestSRLTypesResolveToEmbeddedTopologies(t *testing.T) {
	for nodeType, file := range srlTypes {
		topology, found := srlTopologies[file]
		if !found {
			t.Errorf("type %q points at %q which is not an embedded topology", nodeType, file)
			continue
		}

		if topology.Chassis.Type == 0 || topology.Chassis.CPMCard == 0 {
			t.Errorf("type %q has an incomplete chassis configuration in %q", nodeType, file)
		}

		if len(topology.Slots) == 0 {
			t.Errorf("type %q has no slot configuration in %q", nodeType, file)
		}
	}
}

func TestSRLModularVariants(t *testing.T) {
	for nodeType, want := range modularVariants {
		file, found := srlTypes[nodeType]
		if !found {
			t.Errorf("modular type %q is not registered", nodeType)
			continue
		}

		got := srlTopologies[file]

		if !got.Modular {
			t.Errorf("%s: modular = false, want true", nodeType)
		}

		if got.IMM != want.imm {
			t.Errorf("%s: imm = %q, want %q", nodeType, got.IMM, want.imm)
		}

		if got.ChassisName != want.chassisName {
			t.Errorf("%s: chassis_name = %q, want %q", nodeType, got.ChassisName, want.chassisName)
		}

		if got.CPM != want.cpm {
			t.Errorf("%s: cpm = %q, want %q", nodeType, got.CPM, want.cpm)
		}

		if got.ChassisMode != want.chassisMode {
			t.Errorf("%s: chassis_mode = %q, want %q", nodeType, got.ChassisMode, want.chassisMode)
		}

		if got.Chassis.Type != want.chassisType {
			t.Errorf("%s: chassis_type = %d, want %d",
				nodeType, got.Chassis.Type, want.chassisType)
		}

		if got.Chassis.CPMCard != want.cpmCardType {
			t.Errorf("%s: cpm_card_type = %d, want %d",
				nodeType, got.Chassis.CPMCard, want.cpmCardType)
		}

		// resolving components relies on every modular variant describing its card in slot 1
		slot, found := got.Slots[1]
		if !found {
			t.Errorf("%s: has no slot 1", nodeType)
			continue
		}

		if slot.CardType != want.cardType || slot.MDAType != want.mdaType {
			t.Errorf("%s: slot 1 = card %d / mda %d, want card %d / mda %d",
				nodeType, slot.CardType, slot.MDAType, want.cardType, want.mdaType)
		}
	}
}

// The bare 6e/10e/18e types must keep resolving to the chassis they shipped with, otherwise
// existing labs would come up on different hardware after an upgrade.
func TestSRLBareModularTypesAreUnchanged(t *testing.T) {
	for bare, variant := range map[string]string{
		"ixr-6e":  "ixr-6e-gen2cp-qsfpdd",
		"ixr-10e": "ixr-10e-gen2cp-qsfpdd",
		"ixr-18e": "ixr-18e-qsfpdd",
	} {
		if srlTypes[bare] != srlTypes[variant] {
			t.Errorf("type %q resolves to %q, want the same file as %q (%q)",
				bare, srlTypes[bare], variant, srlTypes[variant])
		}
	}
}

func TestSRLNonModularTopologiesCarryNoIMM(t *testing.T) {
	for file, topology := range srlTopologies {
		if topology.Modular {
			continue
		}

		if topology.IMM != "" || topology.ChassisMode != "" || topology.CPM != "" ||
			topology.ChassisName != "" {
			t.Errorf("%s: non-modular topology declares modular-only keys", file)
		}
	}
}

func TestResolveSRLTopologyWithComponents(t *testing.T) {
	tests := map[string]struct {
		nodeType    string
		components  []*clabtypes.Component
		wantErr     string
		wantCPM     int
		wantMode    string
		wantSlots   map[int]srlSlot
		wantNoSlots bool
	}{
		"no components falls back to the type default": {
			nodeType:  "ixr-10e",
			wantCPM:   184,
			wantMode:  "GEN2CP_ONLY",
			wantSlots: map[int]srlSlot{1: {CardType: 182, MDAType: 199}},
		},
		"imm selects the card and mda for the chassis": {
			nodeType:   "ixr-10e",
			components: []*clabtypes.Component{{Slot: "1", Type: "imm3-36-800g-osfp"}},
			wantCPM:    184,
			wantMode:   "GEN3_ONLY",
			wantSlots:  map[int]srlSlot{1: {CardType: 53, MDAType: 14}},
		},
		"a card offered by a single chassis resolves with its own cpm": {
			nodeType:   "ixr-18e",
			components: []*clabtypes.Component{{Type: "imm3-36-800g-sync-qsfpdd"}},
			wantCPM:    25,
			wantMode:   "GEN3_ONLY",
			wantSlots:  map[int]srlSlot{1: {CardType: 46, MDAType: 11}},
		},
		"a card offered only by the 6e and 10e is rejected on the 18e": {
			nodeType:   "ixr-18e",
			components: []*clabtypes.Component{{Type: "imm3-36-800g-qsfpdd"}},
			wantErr:    "unknown line card",
		},
		"an imm can pull in a different cpm card": {
			nodeType:   "ixr-6e",
			components: []*clabtypes.Component{{Type: "imm2-36-400g-sync-qsfpdd"}},
			wantCPM:    41,
			wantMode:   "GEN2CP_ONLY",
			wantSlots:  map[int]srlSlot{1: {CardType: 43, MDAType: 199}},
		},
		"multiple line cards populate multiple slots": {
			nodeType: "ixr-10e",
			components: []*clabtypes.Component{
				{Slot: "1", Type: "imm3-36-800g-osfp"},
				{Slot: "3", Type: "imm3-36-800g-osfp"},
			},
			wantCPM:  184,
			wantMode: "GEN3_ONLY",
			wantSlots: map[int]srlSlot{
				1: {CardType: 53, MDAType: 14},
				3: {CardType: 53, MDAType: 14},
			},
		},
		"a fixed chassis rejects components": {
			nodeType:   "ixr-d2l",
			components: []*clabtypes.Component{{Type: "imm36-400g-qsfpdd"}},
			wantErr:    "is not modular",
		},
		"an imm the chassis does not offer is rejected": {
			nodeType:   "ixr-18e",
			components: []*clabtypes.Component{{Type: "imm60-100g-qsfp28"}},
			wantErr:    "unknown line card",
		},
		"two line cards in the same slot are rejected": {
			nodeType: "ixr-6e",
			components: []*clabtypes.Component{
				{Slot: "2", Type: "imm3-36-800g-osfp"},
				{Slot: "2", Type: "imm3-36-800g-osfp"},
			},
			wantErr: "duplicate component slot 2",
		},
		"line cards of different generations cannot share a chassis": {
			nodeType: "ixr-6e",
			components: []*clabtypes.Component{
				{Slot: "1", Type: "imm36-400g-qsfpdd"},
				{Slot: "2", Type: "imm3-36-800g-osfp"},
			},
			wantErr: "cannot share a chassis",
		},
		"a non numeric slot is rejected": {
			nodeType:   "ixr-6e",
			components: []*clabtypes.Component{{Slot: "A", Type: "imm3-36-800g-osfp"}},
			wantErr:    "invalid component slot",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := resolveSRLTopology(&clabtypes.NodeConfig{
				NodeType:   tc.nodeType,
				Components: tc.components,
			})

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Chassis.CPMCard != tc.wantCPM {
				t.Errorf("cpm_card_type = %d, want %d", got.Chassis.CPMCard, tc.wantCPM)
			}

			if got.ChassisMode != tc.wantMode {
				t.Errorf("chassis_mode = %q, want %q", got.ChassisMode, tc.wantMode)
			}

			if len(got.Slots) != len(tc.wantSlots) {
				t.Fatalf("slots = %v, want %v", got.Slots, tc.wantSlots)
			}

			for slot, want := range tc.wantSlots {
				if got.Slots[slot] != want {
					t.Errorf("slot %d = %v, want %v", slot, got.Slots[slot], want)
				}
			}
		})
	}
}

func TestGenerateSRLTopologyFile(t *testing.T) {
	cfg := &clabtypes.NodeConfig{
		NodeType: "ixr-10e",
		LabDir:   t.TempDir(),
		Components: []*clabtypes.Component{
			{Slot: "3", Type: "imm3-36-800g-osfp"},
			{Slot: "1", Type: "imm3-36-800g-osfp"},
		},
	}

	if err := generateSRLTopologyFile(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rendered, err := os.ReadFile(filepath.Join(cfg.LabDir, "topology.yml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `chassis_configuration:
  "chassis_type": 69
  "base_mac": "` + cfg.MacAddress + `"
  "cpm_card_type": 184

slot_configuration:
  1:
    "card_type": 53
    "mda_type": 14
  3:
    "card_type": 53
    "mda_type": 14
`

	if string(rendered) != want {
		t.Errorf("rendered topology:\n%s\nwant:\n%s", rendered, want)
	}

	var parsed srlTopology
	if err := yaml.Unmarshal(rendered, &parsed); err != nil {
		t.Fatalf("rendered topology is not valid yaml: %v", err)
	}
}
