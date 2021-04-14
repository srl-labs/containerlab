package config

import (
	"fmt"

	"github.com/srl-labs/containerlab/clab"
)

type stringMap map[string]string

type ConfigSnippet struct {
	TargetNode *clab.Node
	Data       []byte // the Rendered template

	// some info for tracing/debugging
	templateName, source string
	// All the variables used to render the template
	templateLabels *stringMap
}

type Transport interface {
	// Connect to the target host
	Connect(host string) error
	// Execute some config
	Write(snip *ConfigSnippet) error
	Close()
}

func WriteConfig(transport Transport, snips []*ConfigSnippet) error {
	host := snips[0].TargetNode.LongName

	// the Kind should configure the transport parameters before

	err := transport.Connect(host)
	if err != nil {
		return fmt.Errorf("%s: %s", host, err)
	}

	defer transport.Close()

	for _, snip := range snips {
		err := transport.Write(snip)
		if err != nil {
			return fmt.Errorf("could not write config %s: %s", snip, err)
		}
	}

	return nil
}
