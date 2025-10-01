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
)

// A FailMode is a failure mode which Open vSwitch uses when it cannot
// contact a controller.
type FailMode string

// FailMode constants which can be used in OVS configurations.
const (
	FailModeStandalone FailMode = "standalone"
	FailModeSecure     FailMode = "secure"
)

// An InterfaceType is a network interface type recognized by Open vSwitch.
type InterfaceType string

// InterfaceType constants which can be used in OVS configurations.
const (
	InterfaceTypeGRE      InterfaceType = "gre"
	InterfaceTypeInternal InterfaceType = "internal"
	InterfaceTypePatch    InterfaceType = "patch"
	InterfaceTypeSTT      InterfaceType = "stt"
	InterfaceTypeVXLAN    InterfaceType = "vxlan"
)

// A PortAction is a port actions to change the port characteristics of the
// specific port through the ModPort API.
type PortAction string

// PortAction constants for ModPort API.
const (
	PortActionUp           PortAction = "up"
	PortActionDown         PortAction = "down"
	PortActionSTP          PortAction = "stp"
	PortActionNoSTP        PortAction = "no-stp"
	PortActionReceive      PortAction = "receive"
	PortActionNoReceive    PortAction = "no-receive"
	PortActionReceiveSTP   PortAction = "receive-stp"
	PortActionNoReceiveSTP PortAction = "no-receive-stp"
	PortActionForward      PortAction = "forward"
	PortActionNoForward    PortAction = "no-forward"
	PortActionFlood        PortAction = "flood"
	PortActionNoFlood      PortAction = "no-flood"
	PortActionPacketIn     PortAction = "packet-in"
	PortActionNoPacketIn   PortAction = "no-packet-in"
)

// An Error is an error returned when shelling out to an Open vSwitch control
// program.  It captures the combined stdout and stderr as well as the exit
// code.
type Error struct {
	Out []byte
	Err error
}

// Error returns the string representation of an Error.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Err, string(e.Out))
}

// IsPortNotExist checks if err is of type Error and is caused by asking OVS for
// information regarding a non-existent port.
func IsPortNotExist(err error) bool {
	oerr, ok := err.(*Error)
	if !ok {
		return false
	}

	return bytes.HasPrefix(oerr.Out, []byte("ovs-vsctl: no port named ")) &&
		oerr.Err.Error() == "exit status 1"
}
