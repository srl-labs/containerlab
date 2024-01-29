package srl

import (
	"golang.org/x/mod/semver"
)

var snmpv2Config = `set / system snmp access-group SNMPv2-RO-Community security-level no-auth-no-priv community-entry RO-Community community public
set / system snmp network-instance mgmt admin-state enable`

var snmpv2ConfigPre24_3 = `set / system snmp community public
set / system snmp network-instance mgmt admin-state enable`

// setVersionSpecificParams sets version specific parameters in the template data struct
// to enable/disable version-specific configuration blocks in the config template
// or prepares data to conform to the expected format per specific version.
func (n *srl) setVersionSpecificParams(tplData *srlTemplateData) {
	v := n.swVersion.String()

	// in srlinux >= v23.10+ linuxadmin and admin user ssh keys can only be configured via the cli
	// so we add the keys to the template data for rendering.
	if len(n.sshPubKeys) > 0 && (semver.Compare(v, "v23.10") >= 0 || n.swVersion.major == "0") {
		tplData.SSHPubKeys = catenateKeys(n.sshPubKeys)
	}

	// in srlinux v23.10+ till 24.3 we need to enable GNMI unix socket services to enable
	// communications over unix socket (e.g. NDK agents)
	if semver.Compare(v, "v23.10") >= 0 && semver.Compare(v, "v24.3") < 0 {
		tplData.EnableGNMIUnixSockServices = true
	}

	// in versions <= v24.3 SNMPv2 is done differently
	if semver.Compare(v, "v24.3") < 0 && n.swVersion.major != "0" {
		tplData.SNMPConfig = snmpv2ConfigPre24_3
	}
}
