package sros

import (
	"context"
	"regexp"

	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/clab/exec"
)

const (
	snmpv2Config = `configure    system security snmp community "public" access-permissions r
configure    system security snmp community "public" version v2c
configure    system management-interface snmp packet-size 9216
configure    system management-interface snmp streaming admin-state enable
`

	grpcConfig = `configure    system grpc admin-state enable
configure    system security user-params local-user user "admin" access grpc true
configure    system grpc allow-unsecure-connection
configure    system grpc gnmi auto-config-save true
configure    system grpc rib-api admin-state enable
`

	netconfConfig = `configure    system security user-params local-user user "admin" access netconf true
configure    system management-interface netconf auto-config-save true
`

	loggingConfig = `configure log filter "1001" named-entry "10" description "Collect only events of major severity or higher"
configure log filter "1001" named-entry "10" action forward
configure log filter "1001" named-entry "10" match severity gte major
configure log log-id "99" description "Default System Log"
configure log log-id "99" source main true
configure log log-id "99" destination memory max-entries 500
configure log log-id "100" description "Default Serious Errors Log"
configure log log-id "100" filter "1001"
configure log log-id "100" source main true
configure log log-id "100" destination memory max-entries 500
`

	systemCfg = `configure system security aaa local-profiles profile "administrative" default-action permit-all
configure system security aaa local-profiles profile "administrative" entry 10 match "configure system security"
configure system security aaa local-profiles profile "administrative" entry 10 action permit
configure system security aaa local-profiles profile "administrative" entry 20 match "show system security"
configure system security aaa local-profiles profile "administrative" entry 20 action permit
configure system security aaa local-profiles profile "administrative" entry 30 match "tools perform security"
configure system security aaa local-profiles profile "administrative" entry 30 action permit
configure system security aaa local-profiles profile "administrative" entry 40 match "tools dump security"
configure system security aaa local-profiles profile "administrative" entry 40 action permit
configure system security aaa local-profiles profile "administrative" entry 42 match "tools dump system security"
configure system security aaa local-profiles profile "administrative" entry 42 action permit
configure system security aaa local-profiles profile "administrative" entry 50 match "admin system security"
configure system security aaa local-profiles profile "administrative" entry 50 action permit
configure system security aaa local-profiles profile "administrative" entry 100 match "configure li"
configure system security aaa local-profiles profile "administrative" entry 100 action deny
configure system security aaa local-profiles profile "administrative" entry 110 match "show li"
configure system security aaa local-profiles profile "administrative" entry 110 action deny
configure system security aaa local-profiles profile "administrative" entry 111 match "clear li"
configure system security aaa local-profiles profile "administrative" entry 111 action deny
configure system security aaa local-profiles profile "administrative" entry 112 match "tools dump li"
configure system security aaa local-profiles profile "administrative" entry 112 action deny
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization action true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization cancel-commit true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization close-session true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization commit true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization copy-config true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization create-subscription true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization delete-config true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization discard-changes true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization edit-config true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization get true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization get-config true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization get-data true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization get-schema true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization kill-session true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization lock true
configure system security aaa local-profiles profile "administrative" netconf base-op-authorization validate true
configure system security aaa local-profiles profile "default" entry 10 match "exec"
configure system security aaa local-profiles profile "default" entry 10 action permit
configure system security aaa local-profiles profile "default" entry 20 match "exit"
configure system security aaa local-profiles profile "default" entry 20 action permit
configure system security aaa local-profiles profile "default" entry 30 match "help"
configure system security aaa local-profiles profile "default" entry 30 action permit
configure system security aaa local-profiles profile "default" entry 40 match "logout"
configure system security aaa local-profiles profile "default" entry 40 action permit
configure system security aaa local-profiles profile "default" entry 50 match "password"
configure system security aaa local-profiles profile "default" entry 50 action permit
configure system security aaa local-profiles profile "default" entry 60 match "show config"
configure system security aaa local-profiles profile "default" entry 60 action deny
configure system security aaa local-profiles profile "default" entry 65 match "show li"
configure system security aaa local-profiles profile "default" entry 65 action deny
configure system security aaa local-profiles profile "default" entry 66 match "clear li"
configure system security aaa local-profiles profile "default" entry 66 action deny
configure system security aaa local-profiles profile "default" entry 67 match "tools dump li"
configure system security aaa local-profiles profile "default" entry 67 action deny
configure system security aaa local-profiles profile "default" entry 68 match "state li"
configure system security aaa local-profiles profile "default" entry 68 action deny
configure system security aaa local-profiles profile "default" entry 70 match "show"
configure system security aaa local-profiles profile "default" entry 70 action permit
configure system security aaa local-profiles profile "default" entry 75 match "state"
configure system security aaa local-profiles profile "default" entry 75 action permit
configure system security aaa local-profiles profile "default" entry 80 match "enable-admin"
configure system security aaa local-profiles profile "default" entry 80 action permit
configure system security aaa local-profiles profile "default" entry 90 match "enable"
configure system security aaa local-profiles profile "default" entry 90 action permit
configure system security aaa local-profiles profile "default" entry 100 match "configure li"
configure system security aaa local-profiles profile "default" entry 100 action deny
configure system security ssh server-cipher-list-v2 cipher 190 name aes256-ctr
configure system security ssh server-cipher-list-v2 cipher 192 name aes192-ctr
configure system security ssh server-cipher-list-v2 cipher 194 name aes128-ctr
configure system security ssh server-cipher-list-v2 cipher 200 name aes128-cbc
configure system security ssh server-cipher-list-v2 cipher 205 name 3des-cbc
configure system security ssh server-cipher-list-v2 cipher 225 name aes192-cbc
configure system security ssh server-cipher-list-v2 cipher 230 name aes256-cbc
configure system security ssh client-cipher-list-v2 cipher 190 name aes256-ctr
configure system security ssh client-cipher-list-v2 cipher 192 name aes192-ctr
configure system security ssh client-cipher-list-v2 cipher 194 name aes128-ctr
configure system security ssh client-cipher-list-v2 cipher 200 name aes128-cbc
configure system security ssh client-cipher-list-v2 cipher 205 name 3des-cbc
configure system security ssh client-cipher-list-v2 cipher 225 name aes192-cbc
configure system security ssh client-cipher-list-v2 cipher 230 name aes256-cbc
configure system security ssh server-mac-list-v2 mac 200 name hmac-sha2-512
configure system security ssh server-mac-list-v2 mac 210 name hmac-sha2-256
configure system security ssh server-mac-list-v2 mac 215 name hmac-sha1
configure system security ssh server-mac-list-v2 mac 220 name hmac-sha1-96
configure system security ssh server-mac-list-v2 mac 225 name hmac-md5
configure system security ssh server-mac-list-v2 mac 240 name hmac-md5-96
configure system security ssh client-mac-list-v2 mac 200 name hmac-sha2-512
configure system security ssh client-mac-list-v2 mac 210 name hmac-sha2-256
configure system security ssh client-mac-list-v2 mac 215 name hmac-sha1
configure system security ssh client-mac-list-v2 mac 220 name hmac-sha1-96
configure system security ssh client-mac-list-v2 mac 225 name hmac-md5
configure system security ssh client-mac-list-v2 mac 240 name hmac-md5-96
configure system security user-params local-user user "admin" password "NokiaSros1!"
configure system security user-params local-user user "admin" restricted-to-home false
configure system security user-params local-user user "admin" access console true
configure system security user-params local-user user "admin" console member ["administrative"]`
	// ocServerConfig = `set / system management openconfig admin-state enable`.

)

// SrosVersion represents an SR-OS version as a set of fields.
type SrosVersion struct {
	Major string
	Minor string
	Build string
}

// RunningVersion gets the software version of the running node
// by executing the "cat /etc/sros-version" command
// and parsing the output.
func (n *sros) RunningVersion(ctx context.Context) (*SrosVersion, error) {
	cmd, _ := exec.NewExecCmdFromString(`cat /etc/sros-version`)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return nil, err
	}

	log.Debugf("SR-OS node %s extracted raw version. stdout: %s, stderr: %s",
		n.Cfg.ShortName, execResult.GetStdOutString(), execResult.GetStdErrString())

	return n.parseVersionString(execResult.GetStdOutString()), nil
}

func (*sros) parseVersionString(s string) *SrosVersion {
	re, _ := regexp.Compile(`version: (\d{1,3})\.(\d{1,2})\.(\S+)`)

	v := re.FindStringSubmatch(s)
	// 4 matches must be returned if all goes well
	if len(v) != 4 {
		// return all zeroes if failed to parse
		return &SrosVersion{"0", "0", "0"}
	}
	return &SrosVersion{v[1], v[2], v[3]}
}

// String returns a string representation of the version in a semver fashion (with leading v).
func (v *SrosVersion) String() string {
	return "v" + v.Major + "." + v.Minor + "." + v.Build
}

// MajorMinorSemverString returns a string representation of the major.minor version with a leading v.
func (v *SrosVersion) MajorMinorSemverString() string {
	return "v" + v.Major + "." + v.Minor
}

// setVersionSpecificParams sets version specific parameters in the template data struct
// to enable/disable version-specific configuration blocks in the config template
// or prepares data to conform to the expected format per specific version.
func (n *sros) setVersionSpecificParams(tplData *srosTemplateData) {
	// v is in the vMajor.Minor format
	v := n.swVersion.String()
	log.Debugf("SR-OS node %s", v)
}
