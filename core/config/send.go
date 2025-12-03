package config

import (
	"fmt"

	clabcoreconfigtransport "github.com/srl-labs/containerlab/core/config/transport"
)

func Send(cs *NodeConfig, _ string, debug bool) error {
	var tx clabcoreconfigtransport.Transport

	var err error

	ct, ok := cs.TargetNode.Labels["config.transport"]
	if !ok {
		ct = "ssh"
	}

	switch ct {
	case "ssh":
		ssh_cred := cs.Credentials

		if len(ssh_cred) < 2 { //nolint: mnd
			return fmt.Errorf("SSH credentials for node %s of type %s not found, cannot configure",
				cs.TargetNode.ShortName, cs.TargetNode.Kind)
		}

		opts := []clabcoreconfigtransport.SSHTransportOption{
			clabcoreconfigtransport.WithUserNamePassword(
				ssh_cred[0],
				ssh_cred[1]),
			clabcoreconfigtransport.HostKeyCallback(),
		}

		if debug {
			opts = append(opts, clabcoreconfigtransport.WithDebug())
		}

		tx, err = clabcoreconfigtransport.NewSSHTransport(
			cs.TargetNode,
			opts...,
		)
		if err != nil {
			return err
		}
	case "grpc":
		// NewGRPCTransport
	default:
		return fmt.Errorf("unknown transport: %s", ct)
	}

	err = clabcoreconfigtransport.Write(tx, cs.TargetNode.LongName, cs.Data, cs.Info)
	if err != nil {
		return err
	}

	return nil
}
