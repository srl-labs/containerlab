package srl

// edaDiscoveryServerConfig contains configuration for the EDA discovery server.
const edaDiscoveryServerConfig = `!!! EDA Discovery gRPC server
set / system grpc-server eda-discovery services [ gnmi gnsi ]
set / system grpc-server eda-discovery admin-state enable
set / system grpc-server eda-discovery port 50052
set / system grpc-server eda-discovery rate-limit 65535
set / system grpc-server eda-discovery session-limit 1024
set / system grpc-server eda-discovery metadata-authentication true
set / system grpc-server eda-discovery default-tls-profile true
set / system grpc-server eda-discovery network-instance mgmt

!!! ACL rules allowing incoming tcp/50052 for the eda-discovery grpc server
set / acl acl-filter cpm type ipv4 entry 355 description "Containerlab-added rule: Accept incoming gRPC over port 50052 for the eda-discovery gRPC server"
set / acl acl-filter cpm type ipv4 entry 355 match ipv4 protocol tcp
set / acl acl-filter cpm type ipv4 entry 355 match transport destination-port operator eq
set / acl acl-filter cpm type ipv4 entry 355 match transport destination-port value 50052
set / acl acl-filter cpm type ipv4 entry 355 action accept

set / acl acl-filter cpm type ipv6 entry 365 description "Containerlab-added rule: Accept incoming gRPC over port 50052 for the eda-discovery gRPC server"
set / acl acl-filter cpm type ipv6 entry 365 match ipv6 next-header tcp
set / acl acl-filter cpm type ipv6 entry 365 match transport destination-port operator eq
set / acl acl-filter cpm type ipv6 entry 365 match transport destination-port value 50052
set / acl acl-filter cpm type ipv6 entry 365 action accept`

// edaCustomMgmtServerConfig contains configuration for the EDA management servers
// running over custom ports.
const edaCustomMgmtServerConfig = `!!! EDA Management gRPC server
set / system grpc-server eda-mgmt services [ gnmi gnoi gnsi ]
set / system grpc-server eda-mgmt admin-state enable
set / system grpc-server eda-mgmt port 57410
set / system grpc-server eda-mgmt rate-limit 65535
set / system grpc-server eda-mgmt session-limit 1024
set / system grpc-server eda-mgmt metadata-authentication true
set / system grpc-server eda-mgmt tls-profile EDA
set / system grpc-server eda-mgmt network-instance mgmt

!!! ACL rules allowing incoming tcp/57410 for the eda-discovery grpc server
set / acl acl-filter cpm type ipv4 entry 356 description "Containerlab-added rule: Accept incoming gRPC over port 57410 for the eda-mgmt gRPC server"
set / acl acl-filter cpm type ipv4 entry 356 match ipv4 protocol tcp
set / acl acl-filter cpm type ipv4 entry 356 match transport destination-port operator eq
set / acl acl-filter cpm type ipv4 entry 356 match transport destination-port value 57410
set / acl acl-filter cpm type ipv4 entry 356 action accept

set / acl acl-filter cpm type ipv6 entry 366 description "Containerlab-added rule: Accept incoming gRPC over port 57410 for the eda-mgmt gRPC server"
set / acl acl-filter cpm type ipv6 entry 366 match ipv6 next-header tcp
set / acl acl-filter cpm type ipv6 entry 366 match transport destination-port operator eq
set / acl acl-filter cpm type ipv6 entry 366 match transport destination-port value 57410
set / acl acl-filter cpm type ipv6 entry 366 action accept

!!! EDA Management (insecure) gRPC server
set / system grpc-server eda-insecure-mgmt services [ gnmi gnoi gnsi ]
set / system grpc-server eda-insecure-mgmt admin-state enable
set / system grpc-server eda-insecure-mgmt port 57411
set / system grpc-server eda-insecure-mgmt rate-limit 65535
set / system grpc-server eda-insecure-mgmt session-limit 1024
set / system grpc-server eda-insecure-mgmt metadata-authentication true
set / system grpc-server eda-mgmt network-instance mgmt

!!! ACL rules allowing incoming tcp/57411 for the eda-discovery grpc server
set / acl acl-filter cpm type ipv4 entry 357 description "Containerlab-added rule: Accept incoming gRPC over port 57411 for the eda-mgmt gRPC server"
set / acl acl-filter cpm type ipv4 entry 357 match ipv4 protocol tcp
set / acl acl-filter cpm type ipv4 entry 357 match transport destination-port operator eq
set / acl acl-filter cpm type ipv4 entry 357 match transport destination-port value 57411
set / acl acl-filter cpm type ipv4 entry 357 action accept

set / acl acl-filter cpm type ipv6 entry 367 description "Containerlab-added rule: Accept incoming gRPC over port 57411 for the eda-mgmt gRPC server"
set / acl acl-filter cpm type ipv6 entry 367 match ipv6 next-header tcp
set / acl acl-filter cpm type ipv6 entry 367 match transport destination-port operator eq
set / acl acl-filter cpm type ipv6 entry 367 match transport destination-port value 57411
set / acl acl-filter cpm type ipv6 entry 367 action accept`

// edaDefaultMgmtServerConfig is the configuration blob that sets EDA TLS profile
// for the `mgmt` grpc server running over port 57400,
// it is applied when CLAB_EDA_USE_DEFAULT_GRPC_SERVER is set.
const edaDefaultMgmtServerConfig = `set / system grpc-server mgmt metadata-authentication true
set / system grpc-server mgmt tls-profile EDA`
