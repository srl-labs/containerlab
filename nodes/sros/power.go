package sros

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
