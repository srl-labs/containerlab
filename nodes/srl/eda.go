package srl

import _ "embed"

// edaDiscoveryServerConfig contains configuration for the EDA discovery server.
//
//go:embed eda_configs/discovery_server.cfg
var edaDiscoveryServerConfig string

// edaCustomMgmtServerConfig contains configuration for the EDA management servers
// running over custom ports.
//
//go:embed eda_configs/custom_mgmt_server.cfg
var edaCustomMgmtServerConfig string

// edaDefaultMgmtServerConfig is the configuration blob that sets EDA TLS profile
// for the `mgmt` grpc server running over port 57400,
// it is applied when CLAB_EDA_USE_DEFAULT_GRPC_SERVER is set.
//
//go:embed eda_configs/default_mgmt_server.cfg
var edaDefaultMgmtServerConfig string
