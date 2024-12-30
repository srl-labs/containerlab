package srl

import "os"

// edaConfig contains configuration for the EDA onboarding.
// It includes the eda-discovery grpc server and the eda-mgmt secured grpc server
// along with the ACL rules allowing communication over the ports assigned to these servers.
const edaConfig = `!!! EDA Discovery gRPC server
set / system grpc-server eda-discovery services [ gnmi gnsi ]
set / system grpc-server eda-discovery admin-state enable
set / system grpc-server eda-discovery port 50052
set / system grpc-server eda-discovery rate-limit 65535
set / system grpc-server eda-discovery session-limit 1024
set / system grpc-server eda-discovery metadata-authentication true
set / system grpc-server eda-discovery default-tls-profile true
set / system grpc-server eda-discovery network-instance mgmt

# ACL rules allowing incoming tcp/50052 for the eda-discovery grpc server
set / acl acl-filter cpm type ipv4 entry 355 description "Containerlab-added rule: Accept incoming gRPC over port 50052 for the eda-discovery gRPC server"
set / acl acl-filter cpm type ipv4 entry 355 match ipv4 protocol tcp
set / acl acl-filter cpm type ipv4 entry 355 match transport destination-port operator eq
set / acl acl-filter cpm type ipv4 entry 355 match transport destination-port value 50052
set / acl acl-filter cpm type ipv4 entry 355 action accept

set / acl acl-filter cpm type ipv6 entry 365 description "Containerlab-added rule: Accept incoming gRPC over port 50052 for the eda-discovery gRPC server"
set / acl acl-filter cpm type ipv6 entry 365 match ipv6 next-header tcp
set / acl acl-filter cpm type ipv6 entry 365 match transport destination-port operator eq
set / acl acl-filter cpm type ipv6 entry 365 match transport destination-port value 50052
set / acl acl-filter cpm type ipv6 entry 365 action accept

!!! EDA Management gRPC server
set / system grpc-server mgmt metadata-authentication true
set / system grpc-server mgmt tls-profile EDA
`

// setEdaConfig sets the EDA config if the CLAB_EDA_MODE env var is set to any value.
func (n *srl) setEdaConfig(tplData *srlTemplateData) {
	if os.Getenv("CLAB_EDA_MODE") != "" {
		tplData.EDAConfig = edaConfig
	}
}
