// Copyright 2023 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ip

import (
	"context"
	"fmt"
	"net"
	"strings"

	"sigs.k8s.io/knftables"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/utils"
)

const (
	ipMasqTableName = "cni_plugins_masquerade"
	ipMasqChainName = "masq_checks"
)

// The nftables ipmasq implementation is mostly like the iptables implementation, with
// minor updates to fix a bug (adding `ifname`) and to allow future GC support.
//
// We add a rule for each mapping, with a comment containing a hash of its identifiers,
// so that we can later reliably delete the rules we want. (This is important because in
// edge cases, it's possible the plugin might see "ADD container A with IP 192.168.1.3",
// followed by "ADD container B with IP 192.168.1.3" followed by "DEL container A with IP
// 192.168.1.3", and we need to make sure that the DEL causes us to delete the rule for
// container A, and not the rule for container B.)
//
// It would be more nftables-y to have a chain with a single rule doing a lookup against a
// set with an element per mapping, rather than having a chain with a rule per mapping.
// But there's no easy, non-racy way to say "delete the element 192.168.1.3 from the set,
// but only if it was added for container A, not if it was added for container B".

// hashForNetwork returns a unique hash for this network
func hashForNetwork(network string) string {
	return utils.MustFormatHashWithPrefix(16, "", network)
}

// hashForInstance returns a unique hash identifying the rules for this
// network/ifname/containerID
func hashForInstance(network, ifname, containerID string) string {
	return hashForNetwork(network) + "-" + utils.MustFormatHashWithPrefix(16, "", ifname+":"+containerID)
}

// commentForInstance returns a comment string that begins with a unique hash and
// ends with a (possibly-truncated) human-readable description.
func commentForInstance(network, ifname, containerID string) string {
	comment := fmt.Sprintf("%s, net: %s, if: %s, id: %s",
		hashForInstance(network, ifname, containerID),
		strings.ReplaceAll(network, `"`, ``),
		strings.ReplaceAll(ifname, `"`, ``),
		strings.ReplaceAll(containerID, `"`, ``),
	)
	if len(comment) > knftables.CommentLengthMax {
		comment = comment[:knftables.CommentLengthMax]
	}
	return comment
}

// setupIPMasqNFTables is the nftables-based implementation of SetupIPMasqForNetworks
func setupIPMasqNFTables(ipns []*net.IPNet, network, ifname, containerID string) error {
	nft, err := knftables.New(knftables.InetFamily, ipMasqTableName)
	if err != nil {
		return err
	}
	return setupIPMasqNFTablesWithInterface(nft, ipns, network, ifname, containerID)
}

func setupIPMasqNFTablesWithInterface(nft knftables.Interface, ipns []*net.IPNet, network, ifname, containerID string) error {
	staleRules, err := findRules(nft, hashForInstance(network, ifname, containerID))
	if err != nil {
		return err
	}

	tx := nft.NewTransaction()

	// Ensure that our table and chains exist.
	tx.Add(&knftables.Table{
		Comment: knftables.PtrTo("Masquerading for plugins from github.com/containernetworking/plugins"),
	})
	tx.Add(&knftables.Chain{
		Name:    ipMasqChainName,
		Comment: knftables.PtrTo("Masquerade traffic from certain IPs to any (non-multicast) IP outside their subnet"),
	})

	// Ensure that the postrouting chain exists and has the correct rules. (Has to be
	// done after creating ipMasqChainName, so we can jump to it.)
	tx.Add(&knftables.Chain{
		Name:     "postrouting",
		Type:     knftables.PtrTo(knftables.NATType),
		Hook:     knftables.PtrTo(knftables.PostroutingHook),
		Priority: knftables.PtrTo(knftables.SNATPriority),
	})
	tx.Flush(&knftables.Chain{
		Name: "postrouting",
	})
	tx.Add(&knftables.Rule{
		Chain: "postrouting",
		Rule:  "ip daddr == 224.0.0.0/4  return",
	})
	tx.Add(&knftables.Rule{
		Chain: "postrouting",
		Rule:  "ip6 daddr == ff00::/8  return",
	})
	tx.Add(&knftables.Rule{
		Chain: "postrouting",
		Rule: knftables.Concat(
			"goto", ipMasqChainName,
		),
	})

	// Delete stale rules, add new rules to masquerade chain
	for _, rule := range staleRules {
		tx.Delete(rule)
	}
	for _, ipn := range ipns {
		ip := "ip"
		if ipn.IP.To4() == nil {
			ip = "ip6"
		}

		// e.g. if ipn is "192.168.1.4/24", then dstNet is "192.168.1.0/24"
		dstNet := &net.IPNet{IP: ipn.IP.Mask(ipn.Mask), Mask: ipn.Mask}

		tx.Add(&knftables.Rule{
			Chain: ipMasqChainName,
			Rule: knftables.Concat(
				ip, "saddr", "==", ipn.IP,
				ip, "daddr", "!=", dstNet,
				"masquerade",
			),
			Comment: knftables.PtrTo(commentForInstance(network, ifname, containerID)),
		})
	}

	return nft.Run(context.TODO(), tx)
}

// teardownIPMasqNFTables is the nftables-based implementation of TeardownIPMasqForNetworks
func teardownIPMasqNFTables(ipns []*net.IPNet, network, ifname, containerID string) error {
	nft, err := knftables.New(knftables.InetFamily, ipMasqTableName)
	if err != nil {
		return err
	}
	return teardownIPMasqNFTablesWithInterface(nft, ipns, network, ifname, containerID)
}

func teardownIPMasqNFTablesWithInterface(nft knftables.Interface, _ []*net.IPNet, network, ifname, containerID string) error {
	rules, err := findRules(nft, hashForInstance(network, ifname, containerID))
	if err != nil {
		return err
	} else if len(rules) == 0 {
		return nil
	}

	tx := nft.NewTransaction()
	for _, rule := range rules {
		tx.Delete(rule)
	}
	return nft.Run(context.TODO(), tx)
}

// gcIPMasqNFTables is the nftables-based implementation of GCIPMasqForNetwork
func gcIPMasqNFTables(network string, attachments []types.GCAttachment) error {
	nft, err := knftables.New(knftables.InetFamily, ipMasqTableName)
	if err != nil {
		return err
	}
	return gcIPMasqNFTablesWithInterface(nft, network, attachments)
}

func gcIPMasqNFTablesWithInterface(nft knftables.Interface, network string, attachments []types.GCAttachment) error {
	// Find all rules for the network
	rules, err := findRules(nft, hashForNetwork(network))
	if err != nil {
		return err
	} else if len(rules) == 0 {
		return nil
	}

	// Compute the comments for all elements of attachments
	validAttachments := map[string]bool{}
	for _, attachment := range attachments {
		validAttachments[commentForInstance(network, attachment.IfName, attachment.ContainerID)] = true
	}

	// Delete anything in rules that isn't in validAttachments
	tx := nft.NewTransaction()
	for _, rule := range rules {
		if !validAttachments[*rule.Comment] {
			tx.Delete(rule)
		}
	}
	return nft.Run(context.TODO(), tx)
}

// findRules finds rules with comments that start with commentPrefix.
func findRules(nft knftables.Interface, commentPrefix string) ([]*knftables.Rule, error) {
	rules, err := nft.ListRules(context.TODO(), ipMasqChainName)
	if err != nil {
		if knftables.IsNotFound(err) {
			// If ipMasqChainName doesn't exist yet, that's fine
			return nil, nil
		}
		return nil, err
	}

	matchingRules := make([]*knftables.Rule, 0, 1)
	for _, rule := range rules {
		if rule.Comment != nil && strings.HasPrefix(*rule.Comment, commentPrefix) {
			matchingRules = append(matchingRules, rule)
		}
	}

	return matchingRules, nil
}
