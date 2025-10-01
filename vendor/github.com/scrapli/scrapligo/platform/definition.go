package platform

import (
	"fmt"
	"strings"

	"github.com/scrapli/scrapligo/assets"

	"github.com/scrapli/scrapligo/driver/generic"
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/driver/options"

	"github.com/scrapli/scrapligo/util"

	"gopkg.in/yaml.v3"
)

const (
	// AristaEos is a constant representing the platform string/name for Arista EOS devices.
	AristaEos = "arista_eos"
	// ArubaWlc is a constant representing the platform string/name for Aruba Wireless controller
	// AOS devices.
	ArubaWlc = "aruba_wlc"
	// CiscoIosxe is a constant representing the platform string/name for Cisco IOSXE devices.
	CiscoIosxe = "cisco_iosxe"
	// CiscoIosxr is a constant representing the platform string/name for Cisco IOSXR devices.
	CiscoIosxr = "cisco_iosxr"
	// CiscoNxos is a constant representing the platform string/name for Cisco NXOS devices.
	CiscoNxos = "cisco_nxos"
	// CumulusLinux is a constant representing the platform string/name for Cumulus devices.
	CumulusLinux = "cumulus_linux"
	// CumulusVtysh is a constant representing the platform string/name for Cumulus vtysh devices.
	CumulusVtysh = "cumulus_vtysh"
	// HpComware is a constant representing the platform string/name for H3C devices.
	HpComware = "hp_comware"
	// HuaweiVrp is a constant representing the platform string/name for Huawei VRP devices.
	HuaweiVrp = "huawei_vrp"
	// IpinfusionOcnos is a constant representing the platform string/name for Ipinfusion OCNos
	// devices.
	IpinfusionOcnos = "ipinfusion_ocnos"
	// JuniperJunos is a constant representing the platform string/name for Juniper JunOS devices.
	JuniperJunos = "juniper_junos"
	// NokiaSrl is a constant representing the platform string/name for Nokia SRL/SRLinux devices.
	NokiaSrl = "nokia_srl"
	// NokiaSros is a constant representing the platform string/name for Nokia SROS devices.
	NokiaSros = "nokia_sros"
	// NokiaSrosClassic is a constant representing the platform string/name for Nokia SROS devices
	// in classic mode.
	NokiaSrosClassic = "nokia_sros_classic"
	// PaloAltoPanos is a constant representing the platform string/name for Palo Alto PanOS
	// devices.
	PaloAltoPanos = "paloalto_panos"
	// RuijieRgos is a constant representing the platform string/name for Ruijie network devices.
	RuijieRgos = "ruijie_rgos"
	// VyattaVyos is a constant representing the platform string/name for Vyos devices.
	VyattaVyos = "vyatta_vyos"
)

// GetPlatformNames is used to get the "core" (as in embedded in assets and used in testing)
// platform names.
func GetPlatformNames() []string {
	return []string{
		AristaEos,
		ArubaWlc,
		CiscoIosxe,
		CiscoIosxr,
		CiscoNxos,
		CumulusLinux,
		CumulusVtysh,
		HpComware,
		HuaweiVrp,
		IpinfusionOcnos,
		JuniperJunos,
		NokiaSrl,
		NokiaSros,
		NokiaSrosClassic,
		PaloAltoPanos,
		RuijieRgos,
		VyattaVyos,
	}
}

func loadPlatformDefinitionFromAssets(f string) ([]byte, error) {
	if !strings.HasSuffix(f, ".yaml") {
		f += ".yaml"
	}

	return assets.Assets.ReadFile(fmt.Sprintf("platforms/%s", f))
}

func loadPlatformDefinition(f string) (*Definition, error) {
	b, err := loadPlatformDefinitionFromAssets(f)
	if err != nil {
		b, err = util.ResolveAtFileOrURL(f)
		if err != nil {
			return nil, err
		}
	}

	return loadPlatformDefinitionFromBytes(b)
}

func loadPlatformDefinitionFromBytes(b []byte) (*Definition, error) {
	pd := &Definition{}

	err := yaml.Unmarshal(b, pd)
	if err != nil {
		return nil, err
	}

	return pd, nil
}

func setDriver(host string, p *Platform, opts ...util.Option) error {
	finalOpts := p.AsOptions()
	finalOpts = append(finalOpts, opts...)

	var err error

	switch p.DriverType {
	case "generic":
		var d *generic.Driver

		d, err = generic.NewDriver(host, finalOpts...)
		if err != nil {
			return err
		}

		p.genericDriver = d
	case "network":
		var d *network.Driver

		d, err = network.NewDriver(host, finalOpts...)
		if err != nil {
			return err
		}

		p.networkDriver = d
	}

	return err
}

// NewPlatformVariant returns an instance of Platform from the platform definition f where f may be
// a string representing a filepath or URL, or a byte slice of an already loaded YAML definition.
// The provided variant data is merged back into the "base" platform definition. The host and
// any provided options are stored and will be applied when fetching the generic or network driver
// via the GetGenericDriver or GetNetworkDriver methods.
func NewPlatformVariant(
	f interface{},
	variant, host string,
	opts ...util.Option,
) (*Platform, error) {
	var pd *Definition

	var err error

	switch t := f.(type) {
	case string:
		pd, err = loadPlatformDefinition(t)
	case []byte:
		pd, err = loadPlatformDefinitionFromBytes(t)
	}

	if err != nil {
		return nil, err
	}

	p := pd.Default

	vp, ok := pd.Variants[variant]
	if !ok {
		return nil, fmt.Errorf("%w: no variant '%s' in platform", util.ErrPlatformError, variant)
	}

	p.mergeVariant(vp)

	err = setDriver(host, p, opts...)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// NewPlatform returns an instance of Platform from the platform definition f where f may be
// a string representing a filepath or URL, or a byte slice of an already loaded YAML definition.
// The host and any provided options are stored and will be applied when fetching the generic or
// network driver via the GetGenericDriver or GetNetworkDriver methods.
func NewPlatform(f interface{}, host string, opts ...util.Option) (*Platform, error) {
	var pd *Definition

	var err error

	switch t := f.(type) {
	case string:
		pd, err = loadPlatformDefinition(t)
	case []byte:
		pd, err = loadPlatformDefinitionFromBytes(t)
	}

	if err != nil {
		return nil, err
	}

	p := pd.Default
	p.platformType = pd.PlatformType

	err = setDriver(host, p, opts...)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Definition is a struct representing a JSON or YAML platform definition file.
type Definition struct {
	PlatformType string               `json:"platform-type" yaml:"platform-type"`
	Default      *Platform            `json:"default"       yaml:"default"`
	Variants     map[string]*Platform `json:"variants"      yaml:"variants"`
}

// Platform is a struct that contains JSON or YAML data that represent the attributes required to
// create a generic or network driver to connect to a given device type.
type Platform struct {
	platformType string

	// DriverType generic||network
	DriverType string `json:"driver-type" yaml:"driver-type"`

	FailedWhenContains []string       `json:"failed-when-contains" yaml:"failed-when-contains"`
	OnOpen             onXDefinitions `json:"on-open"              yaml:"on-open"`
	OnClose            onXDefinitions `json:"on-close"             yaml:"on-close"`

	PrivilegeLevels              network.PrivilegeLevels `json:"privilege-levels"                yaml:"privilege-levels"`
	DefaultDesiredPrivilegeLevel string                  `json:"default-desired-privilege-level" yaml:"default-desired-privilege-level"`
	NetworkOnOpen                onXDefinitions          `json:"network-on-open"                 yaml:"network-on-open"`
	NetworkOnClose               onXDefinitions          `json:"network-on-close"                yaml:"network-on-close"`

	Options optionDefinitions `json:"options" yaml:"options"`

	genericDriver *generic.Driver
	networkDriver *network.Driver
}

func (p *Platform) mergeVariant(v *Platform) {
	if v.DriverType != "" {
		p.DriverType = v.DriverType
	}

	if len(v.FailedWhenContains) > 0 {
		p.FailedWhenContains = v.FailedWhenContains
	}

	if v.OnOpen != nil {
		p.OnOpen = v.OnOpen
	}

	if v.OnClose != nil {
		p.OnClose = v.OnClose
	}

	if len(v.PrivilegeLevels) > 0 {
		p.PrivilegeLevels = v.PrivilegeLevels
	}

	if v.DefaultDesiredPrivilegeLevel != "" {
		p.DefaultDesiredPrivilegeLevel = v.DefaultDesiredPrivilegeLevel
	}

	if v.NetworkOnOpen != nil {
		p.NetworkOnOpen = v.NetworkOnOpen
	}

	if v.NetworkOnClose != nil {
		p.NetworkOnClose = v.NetworkOnClose
	}
}

// GetGenericDriver returns an instance of generic.Driver built from the Platform data. If the
// platform data (JSON/YAML) specifies a network driver type this will return an error.
func (p *Platform) GetGenericDriver() (*generic.Driver, error) {
	if p.genericDriver == nil {
		return nil, fmt.Errorf(
			"%w: requested generic driver, but generic driver is nil",
			util.ErrPlatformError,
		)
	}

	return p.genericDriver, nil
}

// GetNetworkDriver returns an instance of network.Driver built from the Platform data. If the
// platform data (JSON/YAML) specifies a generic driver type this will return an error.
func (p *Platform) GetNetworkDriver() (*network.Driver, error) {
	if p.networkDriver == nil {
		return nil, fmt.Errorf(
			"%w: requested network driver, but network driver is nil",
			util.ErrPlatformError,
		)
	}

	return p.networkDriver, nil
}

func (p *Platform) genericOptions() []util.Option {
	opts := make([]util.Option, 0)

	if len(p.FailedWhenContains) > 0 {
		opts = append(opts, options.WithFailedWhenContains(p.FailedWhenContains))
	}

	if len(p.OnOpen) > 0 {
		opts = append(opts, options.WithOnOpen(p.OnOpen.asGenericOnX()))
	}

	if len(p.OnClose) > 0 {
		opts = append(opts, options.WithOnClose(p.OnClose.asGenericOnX()))
	}

	return opts
}

// AsOptions returns a slice of options that the platform represents.
func (p *Platform) AsOptions() []util.Option {
	opts := p.genericOptions()

	opts = append(
		opts,
		options.WithPrivilegeLevels(p.PrivilegeLevels),
		options.WithDefaultDesiredPriv(p.DefaultDesiredPrivilegeLevel),
	)

	if len(p.NetworkOnOpen) > 0 {
		opts = append(opts, options.WithNetworkOnOpen(p.NetworkOnOpen.asNetworkOnX()))
	}

	if len(p.NetworkOnClose) > 0 {
		opts = append(opts, options.WithNetworkOnClose(p.NetworkOnClose.asNetworkOnX()))
	}

	opts = append(opts, p.Options.asOptions()...)

	return opts
}

// GetPlatformType returns the string name of the platform definition/type, i.e. "cisco_iosxe" or
// "nokia_srl".
func (p *Platform) GetPlatformType() string {
	return p.platformType
}
