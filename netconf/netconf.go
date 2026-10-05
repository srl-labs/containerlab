// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package netconf contains netconf-based utility functions used in containerlab.
package netconf

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/go-xmlfmt/xmlfmt"

	scrapligocli "github.com/scrapli/scrapligo/v2/cli"
	scrapligonetconf "github.com/scrapli/scrapligo/v2/netconf"
	scrapligooptions "github.com/scrapli/scrapligo/v2/options"
)

const (
	netconfPort = 830
	cliPort     = 22
)

// SaveRunningConfig saves the running config to the startup by means
// of invoking a netconf rpc <copy-config> from running to startup datastore
// this method is used on the network elements that can't perform configuration save via other
// means.
func SaveRunningConfig(ctx context.Context, addr, username, password string) error {
	n, err := scrapligonetconf.NewNetconf(
		addr,
		scrapligooptions.WithPort(netconfPort),
		scrapligooptions.WithUsername(username),
		scrapligooptions.WithPassword(password),
		scrapligooptions.WithTransportSSH2(),
	)
	if err != nil {
		return fmt.Errorf("could not create netconf driver for %s: %+v", addr, err)
	}

	if _, err := n.Open(ctx); err != nil {
		return fmt.Errorf("failed to open netconf driver for %s: %+v", addr, err)
	}
	defer n.Close(ctx)

	r, err := n.CopyConfig(ctx,
		scrapligonetconf.WithSourceType(scrapligonetconf.DatastoreTypeRunning),
		scrapligonetconf.WithTargetType(scrapligonetconf.DatastoreTypeStartup),
	)
	if err != nil {
		return fmt.Errorf("%s: Could not send save config via Netconf: %+v", addr, err)
	}
	if r.Failed {
		return fmt.Errorf("%s: netconf copy-config rpc failed: %s", addr, r.Result)
	}

	return nil
}

// runningConfigCmds maps a scrapli platform name to the command that renders
// the running configuration over the CLI.
var runningConfigCmds = map[string]string{
	"arista_eos":    "show running-config",
	"cisco_asa":     "show running-config",
	"cisco_iosxe":   "show running-config",
	"cisco_iosxr":   "show running-config",
	"cisco_nxos":    "show running-config",
	"juniper_junos": "show configuration",
}

// GetConfig retrieves the running configuration and returns it as a string. It automatically picks
// the appropriate command for the provided Scrapli Platform.
func GetConfig(
	ctx context.Context,
	addr, username, password, scrapliPlatform string,
) (string, error) {
	cmd, ok := runningConfigCmds[scrapliPlatform]
	if !ok {
		return "", fmt.Errorf("retrieving the running config of the %q platform is not supported",
			scrapliPlatform)
	}

	c, err := scrapligocli.NewCli(
		addr,
		scrapligooptions.WithDefinitionFileOrName(scrapliPlatform),
		scrapligooptions.WithPort(cliPort),
		scrapligooptions.WithUsername(username),
		scrapligooptions.WithPassword(password),
		scrapligooptions.WithTransportSSH2(),
	)
	if err != nil {
		return "", fmt.Errorf("could not create cli session for %s: %+v", addr, err)
	}

	if _, err := c.Open(ctx); err != nil {
		return "", fmt.Errorf("failed to open cli session for %s: %+v", addr, err)
	}
	defer c.Close(ctx)

	res, err := c.SendInput(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve config via scrapli for %s: %+v", addr, err)
	}
	if res.Failed() {
		return "", fmt.Errorf(
			"failed to retrieve config via scrapli for %s: %s",
			addr,
			res.Result(),
		)
	}

	log.Debug("Retrieved node config via scrapli", "config", res.Result())

	return res.Result(), nil
}

// Operation defines a NETCONF action to be executed against an established NETCONF session.
type Operation func(*scrapligonetconf.Netconf) (*scrapligonetconf.Result, error)

// MultiExec opens a NETCONF session to the provided address and executes the supplied operations
// sequentially. The driver is opened once and used across every operation, enabling scenarios that
// require multiple NETCONF calls within a single session (for example, chaining import actions
// prior
// to committing configuration changes).
func MultiExec(ctx context.Context, addr, username, password string, operations []Operation) error {
	n, err := scrapligonetconf.NewNetconf(
		addr,
		scrapligooptions.WithPort(netconfPort),
		scrapligooptions.WithUsername(username),
		scrapligooptions.WithPassword(password),
		scrapligooptions.WithTransportSSH2(),
	)
	if err != nil {
		return fmt.Errorf("could not create netconf driver for %s: %+v", addr, err)
	}

	if _, err := n.Open(ctx); err != nil {
		return fmt.Errorf("failed to open netconf driver for %s: %+v", addr, err)
	}
	defer n.Close(ctx)

	for _, operation := range operations {
		r, e := operation(n)
		if e != nil {
			return fmt.Errorf("NETCONF operation failed for %q: %w", addr, e)
		}
		log.Debugf("NETCONF RPC sent to %q: %s", addr,
			xmlfmt.FormatXML(r.Input, "\t", "    "))
		if r.Failed {
			return fmt.Errorf("NETCONF RPC to %q failed: %s",
				addr, r.Result)
		}
	}

	return nil
}
