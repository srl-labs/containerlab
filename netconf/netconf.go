// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package netconf contains netconf-based functions used in containerlab.
package netconf

import (
	"fmt"
	"time"

	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/driver/options"
	scraplilogging "github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/platform"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
	log "github.com/sirupsen/logrus"
)

// SaveConfig saves the running config to the startup by means
// of invoking a netconf rpc <copy-config> from running to startup datastore
// this method is used on the network elements that can't perform configuration save via other means.
func SaveConfig(addr, username, password, _ string) error {
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

func ApplyConfig(addr, platformName, username, password string, config []string) error {
	var err error
	var d *network.Driver
	x := 100
	for i := 0; i < x; i++ {

		li, err := scraplilogging.NewInstance(
			scraplilogging.WithLevel("debug"),
			scraplilogging.WithLogger(log.Debugln))
		if err != nil {
			return err
		}

		opts := []util.Option{
			options.WithAuthNoStrictKey(),
			options.WithAuthUsername(username),
			options.WithAuthPassword(password),
			options.WithTransportType(transport.StandardTransport),
			options.WithTimeoutOps(5 * time.Second),
			options.WithLogger(li),
		}

		p, err := platform.NewPlatform(platformName, addr, opts...)
		if err != nil {
			return fmt.Errorf("failed to create platform; error: %+v", err)
		}

		d, err = p.GetNetworkDriver()
		if err != nil {
			return fmt.Errorf("could not create netconf driver for %s: %+v", addr, err)
		}

		err = d.Open()
		if err == nil {
			break
		}
		log.Debugf("not yet ready - %v", err)
		time.Sleep(time.Second * 3)
	}
	if err != nil {
		return fmt.Errorf("failed to open netconf driver for %s: %+v", addr, err)
	}
	defer d.Close()

	mr, err := d.SendConfigs(config)
	if err != nil {
		return fmt.Errorf("failed to send command; error: %+v", err)
	}

	if mr.Failed != nil {
		return fmt.Errorf("response object indicates failure: %+v", mr.Failed)
	}
	return nil
}
