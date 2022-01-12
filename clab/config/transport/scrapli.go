package transport

import (
	"time"

	"github.com/scrapli/scrapligo/driver/base"
	"github.com/scrapli/scrapligo/driver/core"
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

// Scrapligo native platform names per each kind
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

func NewScrapliTransport(host, kind string, t *types.ConfigTransport) (*network.Driver, error) {
	if t.Scrapli != nil {
		// setting default values
		if t.Scrapli.Port == 0 {
			t.Scrapli.Port = 22
		}

		driver, err := core.NewCoreDriver(
			host,
			NetworkDriver[kind],
			base.WithPort(t.Scrapli.Port),
			base.WithAuthStrictKey(t.Scrapli.AuthStrictKey),
			base.WithTimeoutOps(5*time.Second),
			base.WithSSHConfigFile(t.Scrapli.SSHConfigFile),
			base.WithAuthUsername(t.Scrapli.AuthUsername),
			base.WithAuthPassword(t.Scrapli.AuthPassword),
			base.WithAuthSecondary(t.Scrapli.AuthSecondary),
			base.WithAuthPrivateKey(t.Scrapli.AuthPrivateKey),
		)
		if err != nil {
			return nil, err
		}

		return driver, nil
	} else {
		driver, err := core.NewCoreDriver(
			host,
			NetworkDriver[kind],
			base.WithAuthStrictKey(false),
			base.WithTimeoutOps(5*time.Second),
			base.WithAuthUsername(nodes.DefaultCredentials[kind][0]),
			base.WithAuthPassword(nodes.DefaultCredentials[kind][1]),
		)
		if err != nil {
			return nil, err
		}
		return driver, nil
	}
}
