package config

import (
	"strings"
)

// Split a string on commans and trim each
func SplitTrim(s string) []string {
	res := strings.Split(s, ",")
	for i, v := range res {
		res[i] = strings.Trim(v, " \n\t")
	}
	return res
}

// the new agreed node config
type NodeSettings struct {
	Vars      map[string]string
	Transport string
	Templates []string
}

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
