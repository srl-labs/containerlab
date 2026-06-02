// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import _ "embed"

// Embedded service config snippets (base, IXR, SAR).
// Used by config variant resolution to build startup config.
var (
	//go:embed configs/10_snmpv2.cfg
	snmpv2Config string

	//go:embed configs/11_logging.cfg
	loggingConfig string

	//go:embed configs/12_grpc.cfg
	grpcConfig string

	//go:embed configs/12_grpc_insecure.cfg
	grpcConfigInsecure string

	//go:embed configs/ixr/12_grpc.cfg
	grpcConfigIXR string

	//go:embed configs/ixr/12_grpc_insecure.cfg
	grpcConfigIXRInsecure string

	//go:embed configs/sar/12_grpc.cfg
	grpcConfigSAR string

	//go:embed configs/sar/12_grpc_insecure.cfg
	grpcConfigSARInsecure string

	//go:embed configs/13_netconf.cfg
	netconfConfig string

	//go:embed configs/14_system.cfg
	systemCfg string

	//go:embed configs/ixr/14_system.cfg
	systemCfgIXR string

	//go:embed configs/sar/14_system.cfg
	systemCfgSAR string

	//go:embed configs/15_ssh.cfg
	sshConfig string
)
