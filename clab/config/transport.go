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

// the new agreed node config
type nodeConfig struct {
	Vars      map[string]string
	Transport string
	Templates []string
}

func GetNodeConfigFromLabels(labels map[string]string) nodeConfig {
	nc := nodeConfig{
		Vars:      labels,
		Templates: []string{"base"},
		Transport: "ssh",
	}
	if t, ok := labels["templates"]; ok {
		nc.Templates = strings.Split(t, ",")
		for i, v := range nc.Templates {
			nc.Templates[i] = strings.Trim(v, " \n\t")
		}
	}
	if t, ok := labels["transport"]; ok {
		nc.Transport = t
	}
	return nc
}
