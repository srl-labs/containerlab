package sros

import "golang.org/x/mod/semver"

const (
	grpcMDCLIConfig    = `/configure system grpc md-cli admin-state enable`
	profileMDCLIConfig = `/configure system security aaa local-profiles profile "administrative" grpc rpc-authorization md-cli-session permit`
	tls13CipherCCM8    = `/configure system security tls server-cipher-list "clab-all" tls13-cipher 5 name tls-aes128-ccm8-sha256`
)

func (n *sros) setVersionSpecificParams(tplData *srosTemplateData) {

	currVersion := tplData.SwVersion.MajorMinorSemverString()

	if semver.Compare(currVersion, "v26.0") < 0 {
		tplData.GRPCConfig += "\n" + grpcMDCLIConfig
		tplData.SystemConfig += "\n" + profileMDCLIConfig

		if tplData.IsSecureGrpc {
			tplData.GRPCConfig += "\n" + tls13CipherCCM8
		}
	}
}
