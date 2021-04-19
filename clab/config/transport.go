package config

import (
	"fmt"
	"strings"
)

type Transport interface {
	// Connect to the target host
	Connect(host string) error
	// Execute some config
	Write(snip *ConfigSnippet) error
	Close()
}

func WriteConfig(transport Transport, snips []ConfigSnippet) error {
	host := snips[0].TargetNode.LongName

	// the Kind should configure the transport parameters before

	err := transport.Connect(host)
	if err != nil {
		return fmt.Errorf("%s: %s", host, err)
	}

	defer transport.Close()

	for _, snip := range snips {
		err := transport.Write(&snip)
		if err != nil {
			return fmt.Errorf("could not write config %s: %s", &snip, err)
		}
	}

	return nil
}

// templates to execute
var TemplateOverride string

// the new agreed node config
type NodeConfig struct {
	Vars      map[string]string
	Transport string
	Templates []string
}

// Split a string on commans and trim each
func SplitTrim(s string) []string {
	res := strings.Split(s, ",")
	for i, v := range res {
		res[i] = strings.Trim(v, " \n\t")
	}
	return res
}

func GetNodeConfigFromLabels(labels map[string]string) NodeConfig {
	nc := NodeConfig{
		Vars:      labels,
		Transport: "ssh",
	}
	if TemplateOverride != "" {
		nc.Templates = SplitTrim(TemplateOverride)
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
