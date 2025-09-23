// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package netconf contains netconf-based utility functions used in containerlab.
package netconf

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/go-xmlfmt/xmlfmt"
	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/platform"
	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
	"github.com/scrapli/scrapligocfg"
)

// SaveRunningConfig saves the running config to the startup by means
// of invoking a netconf rpc <copy-config> from running to startup datastore
// this method is used on the network elements that can't perform configuration save via other means.
func SaveRunningConfig(addr, username, password, _ string) error {
	opts := []util.Option{
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(username),
		options.WithAuthPassword(password),
		options.WithTransportType(transport.StandardTransport),
		options.WithPort(830),
	}

	d, err := netconf.NewDriver(
		addr,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("could not create netconf driver for %s: %+v", addr, err)
	}

	err = d.Open()
	if err != nil {
		return fmt.Errorf("failed to open netconf driver for %s: %+v", addr, err)
	}
	defer d.Close()

	_, err = d.CopyConfig("running", "startup")
	if err != nil {
		return fmt.Errorf("%s: Could not send save config via Netconf: %+v", addr, err)
	}

	return nil
}

// GetConfig retrieves the running configuration and returns it as a string. It automatically picks the appropriate network driver for the provided Scrapli Platform.
func GetConfig(addr, username, password, scrapliPlatform string) (string, error) {
	p, err := platform.NewPlatform(
		scrapliPlatform,
		addr,
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(username),
		options.WithAuthPassword(password),
		options.WithTransportType(transport.StandardTransport),
		options.WithPort(22),
	)
	if err != nil {
		return "", fmt.Errorf("could not create or missing platform driver for %s: %+v", addr, err)
	}

	d, err := p.GetNetworkDriver()
	if err != nil {
		return "", fmt.Errorf("could not create generic driver for %s: %+v", addr, err)
	}

	err = d.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open generic driver for %s: %+v", addr, err)
	}
	defer d.Close()

	cfg, err := scrapligocfg.NewCfg(d, scrapliPlatform)
	if err != nil {
		return "", fmt.Errorf("failed to instantiate scrapligocfg for %s: %+v", addr, err)
	}

	err = cfg.Prepare()
	if err != nil {
		return "", fmt.Errorf("failed to prepare scraplicfg connection for %s: %+v", addr, err)
	}

	config, err := cfg.GetConfig("running")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve config via scraplicfg for %s: %+v", addr, err)
	}

	log.Debug("Retrieved node config via scraplicfg", "config", config.Result)

	return config.Result, nil
}

// Operation defines a NETCONF action to be executed against an established NETCONF driver.
type Operation func(*netconf.Driver) (*response.NetconfResponse, error)

// MultiExec opens a NETCONF session to the provided address and executes the supplied operations
// sequentially. The driver is opened once and used across every operation, enabling scenarios that
// require multiple NETCONF calls within a single session (for example, chaining import actions prior
// to committing configuration changes).
func MultiExec(addr, username, password string, operations []Operation) error {
	opts := []util.Option{
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(username),
		options.WithAuthPassword(password),
		options.WithTransportType(transport.StandardTransport),
		options.WithPort(830),
	}

	d, err := netconf.NewDriver(
		addr,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("could not create netconf driver for %s: %+v", addr, err)
	}

	err = d.Open()
	if err != nil {
		return fmt.Errorf("failed to open netconf driver for %s: %+v", addr, err)
	}
	defer d.Close()

	for _, operation := range operations {
		r, e := operation(d)
		log.Debugf("NETCONF RPC sent to %q: %s", addr,
			xmlfmt.FormatXML(string(r.Input), "\t", "    "))
		if r.Failed != nil {
			return fmt.Errorf("NETCONF RPC to %q failed received: %s",
				addr, r.Result)
		}
		if e != nil {
			return e
		}
	}

	return nil
}
