package clab

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// createMysocketTunnels creates internet reachable personal tunnels using mysocket.io
func createMysocketTunnels(ctx context.Context, c *CLab, node *Node) error {
	// remove the existing sockets
	cmd := []string{"/bin/sh", "-c", "mysocketctl socket ls | awk '/clab/ {print $2}' | xargs -n1 mysocketctl socket delete -s"}
	log.Debugf("Running postdeploy mysocketio command %q", cmd)
	_, _, err := c.Exec(ctx, node.ContainerID, cmd)
	if err != nil {
		return fmt.Errorf("failed to remove existing sockets: %v", err)
	}

	for _, n := range c.Nodes {
		if len(n.Publish) == 0 {
			continue
		}
		for _, socket := range n.Publish {
			split := strings.Split(socket, "/")
			if len(split) > 2 {
				log.Warnf("wrong mysocketio publish section %s. should be type/port-number, i.e. tcp/22", socket)
			}
			t := split[0] // type
			p := split[1] // port

			// create socket and get its ID
			cmd := []string{"/bin/sh", "-c", fmt.Sprintf("mysocketctl socket create -t %s -n clab-%s-%s-%s | awk 'NR==4 {print $2}'", t, t, p, n.ShortName)}
			log.Debugf("Running mysocketio command %q", cmd)
			stdout, _, err := c.Exec(ctx, node.ContainerID, cmd)
			if err != nil {
				return fmt.Errorf("failed to create mysocketio socket: %v", err)
			}
			sockID := strings.TrimSpace(string(stdout))

			// create tunnel and get its ID
			cmd = []string{"/bin/sh", "-c", fmt.Sprintf("mysocketctl tunnel create -s %s | awk 'NR==4 {print $4}'", sockID)}
			log.Debugf("Running mysocketio command %q", cmd)
			stdout, _, err = c.Exec(ctx, node.ContainerID, cmd)
			if err != nil {
				return fmt.Errorf("failed to create mysocketio socket: %v", err)
			}
			tunID := strings.TrimSpace(string(stdout))

			// connect tunnel
			cmd = []string{"/bin/sh", "-c", fmt.Sprintf("mysocketctl tunnel connect --host %s -p %s -s %s -t %s > socket-%s-%s-%s.log",
				n.LongName, p, sockID, tunID, n.ShortName, t, p)}
			log.Debugf("Running mysocketio command %q", cmd)
			c.ExecNotWait(ctx, node.ContainerID, cmd)
		}
	}
	return nil
}
