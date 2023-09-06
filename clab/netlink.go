// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// RemoveHostOrBridgeVeth tries to remove veths connected to the host network namespace or a linux bridge
// and does nothing in case they are not found.
func (c *CLab) RemoveHostOrBridgeVeth(l *types.Link) (err error) {
	switch {
	case l.A.Node.Kind == "host" || l.A.Node.Kind == "bridge":
		link, err := utils.LinkByNameOrAlias(l.A.EndpointName)
		if err != nil {
			log.Debugf("Link %q is already gone: %v", l.A.EndpointName, err)
			break
		}

		log.Debugf("Cleaning up virtual wire: %s:%s <--> %s:%s", l.A.Node.ShortName,
			l.A.EndpointName, l.B.Node.ShortName, l.B.EndpointName)

		err = netlink.LinkDel(link)
		if err != nil {
			log.Debugf("Link %q is already gone: %v", l.A.EndpointName, err)
		}
	case l.B.Node.Kind == "host" || l.B.Node.Kind == "bridge":
		link, err := utils.LinkByNameOrAlias(l.B.EndpointName)
		if err != nil {
			log.Debugf("Link %q is already gone: %v", l.B.EndpointName, err)
			break
		}

		log.Debugf("Cleaning up virtual wire: %s:%s <--> %s:%s", l.A.Node.ShortName,
			l.A.EndpointName, l.B.Node.ShortName, l.B.EndpointName)

		err = netlink.LinkDel(link)
		if err != nil {
			log.Debugf("Link %q is already gone: %v", l.B.EndpointName, err)
		}
	}
	return nil
}
