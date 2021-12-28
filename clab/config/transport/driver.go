package transport

import (
	"time"

	"github.com/scrapli/scrapligo/driver/base"
	"github.com/scrapli/scrapligo/driver/core"
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

// ScrapliGo network drivers per each kind
var NetworkDriver = map[string]string{
	"ceos":     "arista_eos",
	"crpd":     "juniper_junos",
	"srl":      "nokia_sros",
	"vr-csr":   "cisco_iosxe",
	"vr-n9kv":  "cisco_nxos",
	"vr-nxos":  "cisco_nxos",
	"vr-pan":   "paloalto_panos",
	"vr-sros":  "nokia_sros",
	"vr-veos":  "arista_eos",
	"vr-vmx":   "juniper_junos",
	"vr-vqfx":  "juniper_junos",
	"vr-xrv":   "cisco_iosxr",
	"vr-xrv9k": "cisco_iosxr",
}

func GetDriver(cs *types.NodeConfig) (*network.Driver, error) {
	d := cs.Driver
	if d != nil {
		driver, err := core.NewCoreDriver(
			cs.LongName,
			NetworkDriver[cs.Kind],
			base.WithPort(d.Port),
			base.WithAuthStrictKey(d.AuthStrictKey),
			base.WithTimeoutOps(5*time.Second),
			base.WithSSHConfigFile(d.SSHConfigFile),
			base.WithAuthUsername(d.AuthUsername),
			base.WithAuthPassword(d.AuthPassword),
			base.WithAuthSecondary(d.AuthSecondary),
			base.WithAuthPrivateKey(d.AuthPrivateKey),
		)
		if err != nil {
			return nil, err
		}
		return driver, nil
	} else {
		driver, err := core.NewCoreDriver(
			cs.LongName,
			NetworkDriver[cs.Kind],
			base.WithPort(22),
			base.WithAuthStrictKey(false),
			base.WithTimeoutOps(5*time.Second),
			base.WithAuthUsername(nodes.DefaultCredentials[cs.Kind][0]),
			base.WithAuthPassword(nodes.DefaultCredentials[cs.Kind][1]),
		)
		if err != nil {
			return nil, err
		}
		return driver, nil
	}
}
