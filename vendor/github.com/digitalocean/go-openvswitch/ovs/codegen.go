// Copyright 2017 DigitalOcean.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovs

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
)

// hwAddrGoString converts a net.HardwareAddr into its Go syntax representation.
func hwAddrGoString(addr net.HardwareAddr) string {
	buf := bytes.NewBufferString("net.HardwareAddr{")
	for i, b := range addr {
		_, _ = buf.WriteString(fmt.Sprintf("0x%02x", b))

		if i != len(addr)-1 {
			_, _ = buf.WriteString(", ")
		}
	}
	_, _ = buf.WriteString("}")

	return buf.String()
}

// ipv4GoString converts a net.IP (IPv4 only) into its Go syntax representation.
func ipv4GoString(ip net.IP) string {
	ip4 := ip.To4()
	if ip4 == nil {
		return `panic("invalid IPv4 address")`
	}

	buf := bytes.NewBufferString("net.IPv4(")
	for i, b := range ip4 {
		_, _ = buf.WriteString(strconv.Itoa(int(b)))

		if i != len(ip4)-1 {
			_, _ = buf.WriteString(", ")
		}
	}
	_, _ = buf.WriteString(")")

	return buf.String()
}

// bprintf is fmt.Sprintf, but it returns a byte slice instead of a string.
func bprintf(format string, a ...interface{}) []byte {
	return []byte(fmt.Sprintf(format, a...))
}
