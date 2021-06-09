package config

import (
	"strings"
)

// Split a string on commas and trim each line
func SplitTrim(s string) []string {
	res := strings.Split(s, ",")
	for i, v := range res {
		res[i] = strings.Trim(v, " \n\t")
	}
	return res
}

// The new agreed node config
type NodeSettings struct {
	Vars      map[string]string
	Transport string
	Templates []string
}

// Temporary function to extract NodeSettings from the Labels
// In the next phase node settings will be added to the clab file
func GetNodeConfigFromLabels(labels map[string]string) NodeSettings {
	nc := NodeSettings{
		Vars:      labels,
		Transport: "ssh",
	}
	if len(TemplateNames) > 0 {
		nc.Templates = TemplateNames
	} else if t, ok := labels["templates"]; ok {
		nc.Templates = SplitTrim(t)
	} else {
		nc.Templates = []string{"base"}
	}
	if t, ok := labels["transport"]; ok {
		nc.Transport = t
	}
	return nc
}
