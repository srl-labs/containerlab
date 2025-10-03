package sros

import (
	"fmt"
	"strings"
)

// SrosPower defines power supply configuration for a given node type.
type SrosPower struct {
	Modules any
	Shelves int
}

// key = type.
var srosPowerConfig = map[string]SrosPower{
	"sr-1s": {
		Modules: map[string]int{
			"ac/hv": 3,
			"dc":    4,
		},
	},
	"sr-1se": {
		Modules: map[string]int{
			"ac/hv": 3,
			"dc":    4,
		},
	},
	"sr-2s": {
		Modules: map[string]int{
			"ac/hv": 3,
			"dc":    4,
		},
	},
	"sr-2se": {
		Modules: map[string]int{
			"ac/hv": 3,
			"dc":    4,
		},
	},
	"sr-7s": {
		Modules: 10,
		Shelves: 2,
	},
	"sr-14s": {
		Modules: 10,
		Shelves: 2,
	},
}

func (n *sros) generatePowerConfig() string {
	nodeType := strings.ToLower(n.Cfg.NodeType)
	if _, ok := srosPowerConfig[nodeType]; !ok {
		return ""
	}

	cfg := srosPowerConfig[nodeType]

	shelves := 1
	if s := cfg.Shelves; s != 0 {
		shelves = s
	}

	modules := 0
	switch m := cfg.Modules.(type) {
	case map[string]int:
		modules = m[defaultSrosPowerType]
	case int:
		modules = m
	}

	shelfType := fmt.Sprintf("ps-a%d-shelf-dc", modules)

	var config strings.Builder

	for s := 1; s <= shelves; s++ {
		config.WriteString(
			fmt.Sprintf(
				"/configure chassis router chassis-number 1 power-shelf %d power-shelf-type %s\n",
				s, shelfType))

		for m := 1; m <= modules; m++ {
			config.WriteString(
				fmt.Sprintf(
					"/configure chassis router chassis-number 1 power-shelf %d power-module %d power-module-type %s\n",
					s,
					m,
					defaultSrosPowerModuleType,
				))
		}
	}

	return config.String()
}
