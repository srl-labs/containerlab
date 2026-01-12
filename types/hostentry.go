package types

import (
	"fmt"
	"strings"
)

type HostEntry struct {
	ip          string
	name        string
	ipversion   IpVersion
	containerId string
	description string
}

func NewHostEntry(ip, name string, ipversion IpVersion) *HostEntry {
	return &HostEntry{
		ip:        ip,
		name:      name,
		ipversion: ipversion,
	}
}

func (h *HostEntry) SetDescription(d string) *HostEntry {
	h.description = d
	return h
}

func (h *HostEntry) SetContainerID(id string) *HostEntry {
	h.containerId = id
	return h
}

func (h *HostEntry) ToHostEntryString() string {
	result := fmt.Sprintf("%s\t%s", h.ip, h.name)

	if len(h.containerId) >= 12 {
		result = fmt.Sprintf("%s %s", result, h.containerId[:12])
	}
	if h.description != "" {
		result = fmt.Sprintf("%s\t# %s", result, h.description)
	}

	return result
}

type HostEntries []*HostEntry //nolint: recvcheck

func (h HostEntries) ToHostsConfig(ipv IpVersion) string {
	sb := strings.Builder{}

	for _, he := range h {
		// if not the requested version is any or the entry matches the requested version, continue
		if ipv != IpVersionAny && ipv != he.ipversion {
			continue
		}

		sb.WriteString(he.ToHostEntryString())
		sb.WriteString("\n")
	}

	return sb.String()
}

func (h *HostEntries) Merge(other HostEntries) {
	*h = append(*h, other...)
}
