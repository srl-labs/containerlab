// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package srl

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/charmbracelet/log"
	clabtypes "github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
)

// srlSlot is the line card populating a single slot of an SR Linux chassis.
type srlSlot struct {
	CardType int `yaml:"card_type"`
	MDAType  int `yaml:"mda_type"`
}

// srlTopology is a parsed file from the embedded topology directory. Modular, ChassisName,
// ChassisMode, CPM and IMM are containerlab-only keys; SR Linux ignores them when it reads the
// rendered file. ChassisName, CPM and IMM carry the names the platform reports for the hardware
// the numeric ids below select, and are asserted against a booted node in the tests.
type srlTopology struct {
	Modular     bool   `yaml:"modular"`
	ChassisName string `yaml:"chassis_name"`
	ChassisMode string `yaml:"chassis_mode"`
	CPM         string `yaml:"cpm"`
	IMM         string `yaml:"imm"`
	Chassis     struct {
		Type    int `yaml:"chassis_type"`
		CPMCard int `yaml:"cpm_card_type"`
	} `yaml:"chassis_configuration"`
	Slots map[int]srlSlot `yaml:"slot_configuration"`
}

var (
	// srlTopologies holds every embedded topology file, keyed by file name.
	srlTopologies = parseSRLTopologies()

	// immsByChassis indexes the modular topologies by chassis type and imm name, so a
	// components block naming an imm resolves to the card/mda pair valid for that chassis.
	immsByChassis = indexSRLIMMs(srlTopologies)
)

func parseSRLTopologies() map[string]srlTopology {
	entries, err := topologies.ReadDir("topology")
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded srl topologies: %v", err))
	}

	parsed := make(map[string]srlTopology, len(entries))

	for _, entry := range entries {
		b, err := topologies.ReadFile("topology/" + entry.Name())
		if err != nil {
			panic(fmt.Sprintf("failed to read embedded srl topology %s: %v", entry.Name(), err))
		}

		var topology srlTopology
		if err := yaml.Unmarshal(b, &topology); err != nil {
			panic(fmt.Sprintf("failed to parse embedded srl topology %s: %v", entry.Name(), err))
		}

		parsed[entry.Name()] = topology
	}

	return parsed
}

func indexSRLIMMs(parsed map[string]srlTopology) map[int]map[string]srlTopology {
	index := map[int]map[string]srlTopology{}

	for _, topology := range parsed {
		if !topology.Modular || topology.IMM == "" {
			continue
		}

		if index[topology.Chassis.Type] == nil {
			index[topology.Chassis.Type] = map[string]srlTopology{}
		}

		index[topology.Chassis.Type][topology.IMM] = topology
	}

	return index
}

// resolveSRLTopology returns the topology to render for a node: the one bound to its type, with
// the slot configuration replaced by the components block when the node declares one.
func resolveSRLTopology(cfg *clabtypes.NodeConfig) (srlTopology, error) {
	base, found := srlTopologies[srlTypes[cfg.NodeType]]
	if !found {
		return srlTopology{}, fmt.Errorf("no embedded topology for srl type %q", cfg.NodeType)
	}

	if len(cfg.Components) == 0 {
		return base, nil
	}

	if !base.Modular {
		return srlTopology{}, fmt.Errorf(
			"srl type %q is not modular and does not accept a components block", cfg.NodeType)
	}

	resolved := base
	resolved.IMM = ""
	resolved.Slots = map[int]srlSlot{}

	for i, component := range cfg.Components {
		slot, err := srlComponentSlot(component.Slot)
		if err != nil {
			return srlTopology{}, err
		}

		if _, duplicate := resolved.Slots[slot]; duplicate {
			return srlTopology{}, fmt.Errorf("duplicate component slot %d", slot)
		}

		variant, err := srlIMMVariant(base.Chassis.Type, cfg.NodeType, component.Type)
		if err != nil {
			return srlTopology{}, err
		}

		resolved.Slots[slot] = variant.Slots[1]

		// the cpm card and the boot mode are properties of the imm generation rather than of
		// the chassis, so imms requiring different ones cannot share a chassis.
		if i == 0 {
			resolved.Chassis.CPMCard = variant.Chassis.CPMCard
			resolved.ChassisMode = variant.ChassisMode

			continue
		}

		if variant.Chassis.CPMCard != resolved.Chassis.CPMCard ||
			variant.ChassisMode != resolved.ChassisMode {
			return srlTopology{}, fmt.Errorf(
				"line card %q cannot share a chassis with the other line cards of node type %q, "+
					"they need a different cpm card or boot mode",
				component.Type, cfg.NodeType)
		}
	}

	return resolved, nil
}

func srlComponentSlot(slot string) (int, error) {
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return 1, nil
	}

	number, err := strconv.Atoi(slot)
	if err != nil || number < 1 {
		return 0, fmt.Errorf(
			"invalid component slot %q, srl line card slots are positive integers", slot)
	}

	return number, nil
}

// srlTopologyTpl renders the topology file SR Linux reads at boot. The containerlab-only keys of
// the embedded files are deliberately left out of the rendered output.
var srlTopologyTpl = template.Must(template.New("clab-srl-topology").Parse(
	`chassis_configuration:
  "chassis_type": {{ .ChassisType }}
  "base_mac": "{{ .MAC }}"
  "cpm_card_type": {{ .CPMCard }}

slot_configuration:
{{- range .Slots }}
  {{ .Slot }}:
    "card_type": {{ .CardType }}
    "mda_type": {{ .MDAType }}
{{- end }}
`))

type srlTopologySlot struct {
	Slot int
	srlSlot
}

type srlTopologyData struct {
	MAC         string
	ChassisType int
	CPMCard     int
	Slots       []srlTopologySlot
}

func generateSRLTopologyFile(cfg *clabtypes.NodeConfig) error {
	topology, err := resolveSRLTopology(cfg)
	if err != nil {
		return err
	}

	data := srlTopologyData{
		MAC:         genMac(cfg).MAC,
		ChassisType: topology.Chassis.Type,
		CPMCard:     topology.Chassis.CPMCard,
	}

	for _, slot := range slices.Sorted(maps.Keys(topology.Slots)) {
		data.Slots = append(data.Slots,
			srlTopologySlot{Slot: slot, srlSlot: topology.Slots[slot]})
	}

	dst := filepath.Join(cfg.LabDir, "topology.yml")

	log.Debug("generating srl topology file", "path", dst, "mac", data.MAC)

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := srlTopologyTpl.Execute(f, data); err != nil {
		return err
	}

	return f.Close()
}

func srlIMMVariant(chassisType int, nodeType, imm string) (srlTopology, error) {
	supported := immsByChassis[chassisType]

	variant, found := supported[strings.TrimSpace(imm)]
	if !found {
		return srlTopology{}, fmt.Errorf(
			"unknown line card %q for srl type %q, supported line cards are %s",
			imm, nodeType, strings.Join(slices.Sorted(maps.Keys(supported)), ", "))
	}

	return variant, nil
}
