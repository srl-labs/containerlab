package network

import (
	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

const (
	defaultConfigurationPrivLevel = "configuration"
)

// SendConfigs sends a list of configs to the device. This method will auto acquire the
// "configuration" privilege level. If your device does *not* have a "configuration" privilege
// level this will fail. If your device has multiple "flavors" of configuration level (i.e.
// exclusive or private) you can acquire that target privilege level for the configuration option
// by passing the opoptions.WithPrivilegeLevel with the requested privilege level set.
func (d *Driver) SendConfigs(
	configs []string,
	opts ...util.Option,
) (*response.MultiResponse, error) {
	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	targetPriv := op.PrivilegeLevel

	if targetPriv == "" {
		targetPriv = defaultConfigurationPrivLevel
	}

	err = d.AcquirePriv(targetPriv)
	if err != nil {
		return nil, err
	}

	return d.Driver.SendCommands(configs, opts...)
}

// SendConfigsFromFile is a convenience wrapper that sends the lines of file f as config lines via
// SendConfigs.
func (d *Driver) SendConfigsFromFile(
	f string,
	opts ...util.Option,
) (*response.MultiResponse, error) {
	c, err := util.LoadFileLines(f)
	if err != nil {
		return nil, err
	}

	return d.SendConfigs(c, opts...)
}
