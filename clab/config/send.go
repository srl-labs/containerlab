package config

import (
	"fmt"

	"github.com/srl-labs/containerlab/clab/config/transport"
	"github.com/srl-labs/containerlab/nodes"
)

func Send(cs *NodeConfig, _ string) error {
	var tx transport.Transport
	var err error

	ct, ok := cs.TargetNode.Labels["config.transport"]
	if !ok {
		ct = "ssh"
	}

	if ct == "ssh" {
		if len(nodes.DefaultCredentials[cs.TargetNode.Kind]) < 2 {
			return fmt.Errorf("SSH credentials for node %s of type %s not found, cannot configure", cs.TargetNode.ShortName, cs.TargetNode.Kind)
		}
		tx, err = transport.NewSSHTransport(
			cs.TargetNode,
			transport.WithUserNamePassword(
				nodes.DefaultCredentials[cs.TargetNode.Kind][0],
				nodes.DefaultCredentials[cs.TargetNode.Kind][1]),
			transport.HostKeyCallback(),
		)
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
