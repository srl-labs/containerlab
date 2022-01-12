package config

import (
	"fmt"

	"github.com/scrapli/scrapligo/cfg"
	"github.com/srl-labs/containerlab/clab/config/transport"
)

func Send(cs *NodeConfig, _ string) error {
	var err error
	ct, ok := cs.TargetNode.Labels["config.transport"]
	if !ok {
		ct = "ssh"
	}

	if ct == "ssh" {
		// check if Kind is a supported scrapligo platform
		_, ok := transport.NetworkDriver[cs.TargetNode.Kind]
		if !ok {
			return nil
		}
		if cs.TargetNode.Config.Transport.Scrapli != nil {
			driver, err := transport.NewScrapliTransport(cs.TargetNode.LongName, cs.TargetNode.Kind, cs.TargetNode.Config.Transport)
			if err != nil {
				return fmt.Errorf("failed to create driver: %v", err)
			}
			err = driver.Open()
			if err != nil {
				return fmt.Errorf("failed to open driver: %v", err)
			}
			defer driver.Close()
			c, err := cfg.NewCfgDriver(
				driver,
				transport.NetworkDriver[cs.TargetNode.Kind],
			)
			if err != nil {
				return fmt.Errorf("failed to create config driver: %v", err)
			}
			prepareErr := c.Prepare()
			if prepareErr != nil {
				return fmt.Errorf("failed running prepare method: %v", prepareErr)
			}
			// this seems a bit clunky, might need cleaned up
			for i1, d1 := range cs.Data {
				if len(cs.Info[i1]) != 0 {
					_, err = c.LoadConfig(
						string(d1),
						false, //don't load replace. Load merge/set instead
					)
					if err != nil {
						return fmt.Errorf("failed to load config: %+v", err)
					}
				}
			}
			_, err = c.CommitConfig()
			if err != nil {
				return fmt.Errorf("failed to commit config: %+v", err)
			}
		}
	} else if ct == "grpc" {
		// NewGRPCTransport
	} else {
		return fmt.Errorf("unknown transport: %s", ct)
	}
	if err != nil {
		return err
	}
	return nil
}
