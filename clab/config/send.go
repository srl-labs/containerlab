package config

import (
	"fmt"

	"github.com/srl-labs/containerlab/clab/config/transport"
)

func Send(cs *NodeConfig, _ string) error {
	var tx transport.Transport
	var err error
	ct, ok := cs.TargetNode.Labels["config.transport"]
	if !ok {
		ct = "ssh"
	}

	if ct == "ssh" {
		tx, err = transport.NewScrapliTransport(cs.TargetNode)
		if err != nil {
			return err
		}
	} else if ct == "grpc" {
		// NewGRPCTransport
	} else {
		return fmt.Errorf("unknown transport: %s", ct)
	}
	err = transport.Write(tx, cs.TargetNode.LongName, cs.Data, cs.Info)
	if err != nil {
		return err
	}
	return nil
}
