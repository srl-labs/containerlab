package types

import "net/netip"

// IPAMProvider selects the management address allocator.
type IPAMProvider string

const (
	IPAMProviderContainerlab IPAMProvider = "containerlab"
	IPAMProviderRuntime      IPAMProvider = "runtime"
)

func (p IPAMProvider) IsValid() bool {
	return p == "" || p == IPAMProviderContainerlab || p == IPAMProviderRuntime
}

// MgmtIPAM configures management address allocation and duplicate detection.
type MgmtIPAM struct {
	Provider IPAMProvider `json:"provider,omitempty" yaml:"provider,omitempty"`
	DAD      *bool        `json:"dad,omitempty" yaml:"dad,omitempty"`
}

// DADEnabled reports whether duplicate address detection is enabled; nil defaults to true.
func (m MgmtIPAM) DADEnabled() bool {
	return m.DAD == nil || *m.DAD
}

// ExistingAddress is an address already assigned to a node in the management
// network, obtained from runtime inspection during apply.
type ExistingAddress struct {
	NodeName    string
	ContainerID string
	Address     netip.Addr
}

// NodeAddresses contains preferred allocation addresses, excluding explicit topology settings.
type NodeAddresses struct {
	IPv4 string `yaml:"ipv4,omitempty" json:"ipv4,omitempty"`
	IPv6 string `yaml:"ipv6,omitempty" json:"ipv6,omitempty"`
}

// AllocationOptions supplies live addresses, preferred addresses, and runtime reservations.
type AllocationOptions struct {
	Existing  []ExistingAddress
	Preferred map[string]NodeAddresses
	Reserved  []netip.Addr
}
