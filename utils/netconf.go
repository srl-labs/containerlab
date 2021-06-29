// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"

	"github.com/scrapli/scrapligo/driver/base"
	"github.com/scrapli/scrapligo/netconf"
	"github.com/scrapli/scrapligo/transport"
)

// SaveCfgViaNetconf saves the running config to the startup by means
// of invoking a netconf rpc <copy-config>
// this method is used on the network elements that can't perform a save of config via other means
func SaveCfgViaNetconf(addr, username, password string) error {
	d, err := netconf.NewNetconfDriver(
		addr,
		base.WithAuthStrictKey(false),
		base.WithAuthUsername(username),
		base.WithAuthPassword(password),
		base.WithTransportType(transport.StandardTransportName),
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
