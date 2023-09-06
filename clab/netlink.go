// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"
	"strings"

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

// GetLinksByNamePrefix returns a list of links whose name matches a prefix.
func GetLinksByNamePrefix(prefix string) ([]netlink.Link, error) {
	// filtered list of interfaces
	if prefix == "" {
		return nil, fmt.Errorf("prefix is not specified")
	}
	var fls []netlink.Link

	ls, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	for _, l := range ls {
		if strings.HasPrefix(l.Attrs().Name, prefix) {
			fls = append(fls, l)
		}
	}
	if len(fls) == 0 {
		return nil, fmt.Errorf("no links found by specified prefix %s", prefix)
	}
	return fls, nil
}
