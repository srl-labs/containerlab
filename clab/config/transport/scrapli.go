package transport

import (
	"fmt"
	"time"

	"github.com/scrapli/scrapligo/cfg"
	"github.com/scrapli/scrapligo/driver/base"
	"github.com/scrapli/scrapligo/driver/core"
	"github.com/scrapli/scrapligo/driver/network"
	log "github.com/sirupsen/logrus"
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

type ScrapliTransport struct {
	Driver  *network.Driver
	Options *ScrapliOptions
}

type ScrapliOptions struct {
	Platform          string
	Port              int
	AuthUsername      string
	AuthPassword      string
	AuthSecondary     string
	AuthPrivateKey    string
	AuthStrictKey     bool
	SSHConfigFile     string
	SSHKnownHostsFile string
}

func NewScrapliTransport(node *types.NodeConfig) (*ScrapliTransport, error) {
	t := node.Config.Transport
	// check if kind is supported in scrapli
	_, ok := NetworkDriver[node.Kind]
	if !ok {
		return nil, fmt.Errorf("unable to find scrapli platform for kind: %v", node.Kind)
	}
	// setting default values
	if t.Scrapli.Port == 0 {
		t.Scrapli.Port = 22
	}
	if t.Scrapli.SSHConfigFile == "" {
		if t.Scrapli.AuthUsername == "" {
			t.Scrapli.AuthUsername = nodes.DefaultCredentials[node.Kind][0]
		}
		if t.Scrapli.AuthPassword == "" {
			t.Scrapli.AuthPassword = nodes.DefaultCredentials[node.Kind][1]
		}
	}
	if node.Kind == "crpd" {
		if t.Scrapli.AuthSecondary == "" {
			t.Scrapli.AuthSecondary = "clab123"
		}
	}
	options := &ScrapliOptions{
		Platform:          NetworkDriver[node.Kind],
		Port:              t.Scrapli.Port,
		AuthUsername:      t.Scrapli.AuthUsername,
		AuthPassword:      t.Scrapli.AuthPassword,
		AuthSecondary:     t.Scrapli.AuthSecondary,
		AuthPrivateKey:    t.Scrapli.AuthPrivateKey,
		AuthStrictKey:     t.Scrapli.AuthStrictKey,
		SSHConfigFile:     t.Scrapli.SSHConfigFile,
		SSHKnownHostsFile: t.Scrapli.SSHKnownHostsFile,
	}
	driver, err := options.NewScrapliDriver(node.LongName)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver: %v", err)
	}
	return &ScrapliTransport{
		Driver:  driver,
		Options: options,
	}, nil
}

func (t *ScrapliOptions) NewScrapliDriver(host string) (*network.Driver, error) {
	d, err := core.NewCoreDriver(
		host,
		t.Platform,
		base.WithTimeoutOps(5*time.Second),
		base.WithPort(t.Port),
		base.WithAuthStrictKey(t.AuthStrictKey),
		base.WithSSHConfigFile(t.SSHConfigFile),
		base.WithAuthUsername(t.AuthUsername),
		base.WithAuthPassword(t.AuthPassword),
		base.WithAuthSecondary(t.AuthSecondary),
		base.WithAuthPrivateKey(t.AuthPrivateKey),
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *ScrapliTransport) Connect(host string, _ ...TransportOption) error {
	err := d.Driver.Open()
	if err != nil {
		return fmt.Errorf("failed to open driver: %v", err)
	}
	log.Infof("Connected to %s\n", host)
	return nil
}

func (t *ScrapliTransport) Write(data, info *string) error {
	if *data == "" {
		return nil
	}
	c, err := cfg.NewCfgDriver(
		t.Driver,
		t.Options.Platform,
	)
	if err != nil {
		return fmt.Errorf("failed to create config driver: %v", err)
	}
	prepareErr := c.Prepare()
	if prepareErr != nil {
		return fmt.Errorf("failed running prepare method: %v", prepareErr)
	}
	_, err = c.LoadConfig(
		string(*data),
		false, //don't load replace. Load merge/set instead
	)
	if err != nil {
		return fmt.Errorf("failed to load config: %+v", err)
	}
	_, err = c.CommitConfig()
	if err != nil {
		return fmt.Errorf("failed to commit config: %+v", err)
	}
	return nil
}

func (t *ScrapliTransport) Close() {
	t.Driver.Close()
	log.Infof("Connection closed successfully")
}
