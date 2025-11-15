# SR OS Node Implementation for Containerlab

This directory contains the implementation of Nokia SR OS (Service Router Operating System) nodes for Containerlab.

## Overview

The SR OS node kind supports deploying Nokia's SR OS network operating system in containerized environments. This implementation handles various SR OS platforms including:

- **SR routers** (sr-, vsr- series)
- **IXR routers** (ixr- series) 
- **SAR routers** (sar- series, including sar-hm variants)

## Key Components

### Core Files

- **`sros.go`**: Main implementation of the SR OS node kind
  - Node lifecycle management (Init, PreDeploy, Deploy, PostDeploy, Delete)
  - Container configuration and management
  - Distributed fabric deployment support
  - Network interface configuration

- **`version.go`**: SR OS version detection and parsing
  - Extracts version from running containers
  - Reads version from image layers without spawning containers
  - Parses various version string formats (e.g., "25.10.R1", "0.0.I8306")

- **`sros_config.go`**: Configuration generation and management
  - Template-based configuration generation
  - Support for both Model-Driven (MD) and Classic CLI modes
  - Node-type specific configuration (IXR, SAR, SR)
  - TLS certificate management
  - gRPC, NETCONF, SSH configuration

## Architecture

### Node Types

The implementation uses helper methods to identify different SR OS platforms:

```go
isIXRNode()    // Matches: ixr-, IXR- (case-insensitive)
isSARNode()    // Matches: sar-, SAR- (case-insensitive)  
isSARHmNode()  // Matches: sar-hm, sar-h, sar-m, SAR-HM, etc.
```

These helpers use compiled regular expressions for efficient, case-insensitive matching with word boundaries.

### Configuration Modes

SR OS supports two configuration modes:

1. **Model-Driven (MD)**: Modern YANG-based configuration (default for SR OS 25+)
2. **Classic CLI**: Traditional CLI configuration mode

The configuration mode is determined by the `SROS_CONFIG_MODE` environment variable and can be:
- `model-driven` (default)
- `classic`
- `mixed` (treated as classic)

**Note**: SAR-Hm nodes only support classic mode and will be automatically switched if configured otherwise.

### Template Selection Logic

Configuration templates are selected based on:

```
┌─────────────────────────────────────────┐
│   Configuration Mode Check              │
└─────────────┬───────────────────────────┘
              │
              ├─ Classic/Mixed Mode
              │  ├─ IXR → cfgTplClassicIxr
              │  ├─ SAR → cfgTplClassicSar
              │  └─ Other → cfgTplClassic
              │
              └─ Model-Driven Mode
                 └─ All → cfgTplSROS25
```

### Node-Type Specific Configurations

Different platform types have specific configuration overrides:

| Platform | System Config | gRPC Config | Special Notes |
|----------|--------------|-------------|---------------|
| IXR | `systemCfgIXR` | `grpcConfigIXR` | Secure/Insecure variants |
| SAR | `systemCfgSAR` | `grpcConfigSAR` | Secure/Insecure variants |
| SAR-Hm | `systemCfgSAR` | N/A | Classic mode only, no gRPC |
| SR/VSR | `systemCfg` | `grpcConfig` | Default configs |

### Version Detection

The implementation can detect SR OS versions through multiple methods:

1. **Image Labels**: Checks for `sros.version` label in container image
2. **Graph Driver**: Reads `/etc/sros-version` directly from overlay2 filesystem layers
3. **Running Container**: Executes `cat /etc/sros-version` in running container

This is done **before** configuration generation to support version-specific templates in the future.

```go
// Version detection flow
getImageSrosVersion() 
  └─> Check image labels
      └─> Read from GraphDriver.Data.UpperDir
          └─> Fallback to layer traversal
              └─> Default to v25.0.0
```

### Template Data Structure

The `srosTemplateData` struct consolidates all information needed for both template selection and execution:

```go
type srosTemplateData struct {
    // Selection criteria
    NodeType          string
    ConfigurationMode string
    SwVersion         *SrosVersion
    IsSecureGrpc      bool
    
    // Certificate data
    TLSKey, TLSCert, TLSAnchor string
    
    // Network configuration
    IFaces      map[string]tplIFace
    DNSServers  []string
    
    // Service configurations
    SystemConfig, GRPCConfig, NetconfConfig string
    // ... and more
}
```

### Distributed SR OS Support

The implementation supports distributed SR OS deployments with multiple components:

- **Base Node**: Orchestrates the deployment
- **CPM (Control Processing Module)**: Control plane (slots A, B)
- **IOM/Linecard**: Data plane (numbered slots)

Configuration generation handles distributed setups by:
1. Generating component-specific configuration
2. Coordinating deployment order (LCs first, then CPMs)
3. Managing component-specific environment variables

## Configuration Flow

```
createSROSConfigFiles()
  │
  ├─> getImageSrosVersion()           # Detect SR OS version
  │
  ├─> prepareConfigTemplateData()     # Prepare template data
  │   ├─> Gather node information
  │   ├─> Load certificates
  │   ├─> Prepare SSH keys
  │   └─> applyNodeTypeSpecificConfig()  # Apply platform-specific configs
  │
  ├─> selectConfigTemplate()          # Choose appropriate template
  │   └─> Returns template based on mode and platform
  │
  └─> addDefaultConfig()              # Generate and apply config
      └─> Execute template with prepared data
```

## Key Features

### Security

- **TLS Certificate Management**: Automatic certificate generation and injection
- **Secure gRPC**: Support for both secure and insecure gRPC configurations
- **SSH Key Injection**: Automatic SSH public key configuration from host

### Networking

- **Management Interface**: Automatic IP assignment and configuration
- **Service Discovery**: `/etc/hosts` population for inter-node communication
- **MTU Configuration**: Configurable MTU for management interfaces
- **TX Offload Disable**: Automatic disabling of TX checksum offload on host veth

### Monitoring

- **Health Checks**: Container health monitoring during PostDeploy
- **Log Monitoring**: Real-time log scanning for errors during boot
- **Ready Detection**: PID-based readiness detection

### Configuration Management

- **Partial Config Support**: Files with `.partial` suffix are merged with defaults
- **Full Config Support**: Complete startup configurations bypass default generation
- **Component Config Generation**: Automatic configuration for distributed deployments
- **Auto-save**: Configuration automatically saved to startup on boot

## Environment Variables

Key environment variables used by SR OS nodes:

- `SROS_CONFIG_MODE`: Configuration mode (model-driven/classic/mixed)
- `NOKIA_SROS_SLOT`: Slot identifier for distributed deployments
- `NOKIA_SROS_NUM_OF_CPMS`: Number of CPMs in distributed setup

## File Organization

```
sros/
├── sros.go                    # Main node implementation
├── sros_config.go            # Configuration generation
├── version.go                # Version detection and parsing
├── sros_test.go             # Unit tests
├── configs/                  # Embedded configuration templates
│   ├── sros_config_sros25.go.tpl      # Model-driven template
│   ├── sros_config_classic.go.tpl     # Classic CLI template
│   ├── ixr/                           # IXR-specific configs
│   │   ├── 12_grpc.cfg
│   │   ├── 12_grpc_insecure.cfg
│   │   └── 14_system.cfg
│   └── sar/                           # SAR-specific configs
│       ├── 12_grpc.cfg
│       ├── 12_grpc_insecure.cfg
│       └── 14_system.cfg
└── README.md                 # This file
```

## Testing

Run unit tests with:

```bash
# Run all tests
go test -v

# Run specific test
go test -v -run TestNodeTypeHelpers

# Run with coverage
go test -cover
```

## Future Enhancements

### Version-Specific Templates

The infrastructure is in place to support different configuration templates based on SR OS version:

```go
// Future implementation
if tplData.SwVersion.Major >= 26 {
    tmpl = cfgTplSROS26
    tplName = "clab-sros-config-sros26"
}
```

### Additional Platform Support

The modular design allows easy addition of new platform types by:
1. Adding new regexp pattern
2. Creating platform-specific helper method
3. Adding platform-specific configuration files
4. Updating `applyNodeTypeSpecificConfig()` logic

## References

- [Containerlab Documentation](https://containerlab.dev/)
- [Nokia SR OS Documentation](https://documentation.nokia.com/)
- [Docker Overlay2 Storage Driver](https://docs.docker.com/storage/storagedriver/overlayfs-driver/)

## Contributing

When making changes to this implementation:

1. **Add tests** for new functionality
2. **Update helper methods** if adding new node type detection
3. **Document** configuration template changes
4. **Run linting**: `golangci-lint run`
5. **Format code**: `gofmt -w *.go`
6. **Verify tests pass**: `go test -v`

## Troubleshooting

### Version Detection Fails

If version detection fails, check:
- Docker daemon is accessible
- Container image has `/etc/sros-version` file
- Permission to access `/var/lib/docker` (for graph driver method)

### Configuration Not Applied

Check:
- Configuration mode is correctly set in environment variables
- Node type is correctly specified
- Startup config file format (partial vs. full)
- Container logs: `docker logs -f <container-name>`

### Distributed Deployment Issues

Verify:
- Component slots are correctly configured# SR OS Node Implementation for Containerlab

This directory contains the implementation of Nokia SR OS (Service Router Operating System) nodes for Containerlab.

## Overview

The SR OS node kind supports deploying Nokia's SR OS network operating system in containerized environments. This implementation handles various SR OS platforms including:

- **SR routers** (sr-, vsr- series)
- **IXR routers** (ixr- series) 
- **SAR routers** (sar- series, including sar-hm variants)

## Key Components

### Core Files

- **`sros.go`**: Main implementation of the SR OS node kind
  - Node lifecycle management (Init, PreDeploy, Deploy, PostDeploy, Delete)
  - Container configuration and management
  - Distributed fabric deployment support
  - Network interface configuration

- **`version.go`**: SR OS version detection and parsing
  - Extracts version from running containers
  - Reads version from image layers without spawning containers
  - Parses various version string formats (e.g., "25.10.R1", "0.0.I8306")

- **`sros_config.go`**: Configuration generation and management
  - Template-based configuration generation
  - Support for both Model-Driven (MD) and Classic CLI modes
  - Node-type specific configuration (IXR, SAR, SR)
  - TLS certificate management
  - gRPC, NETCONF, SSH configuration

## Architecture

### Node Types

The implementation uses helper methods to identify different SR OS platforms:

```go
isIXRNode()    // Matches: ixr-, IXR- (case-insensitive)
isSARNode()    // Matches: sar-, SAR- (case-insensitive)  
isSARHmNode()  // Matches: sar-hm, sar-h, sar-m, SAR-HM, etc.
```

These helpers use compiled regular expressions for efficient, case-insensitive matching with word boundaries.

### Configuration Modes

SR OS supports two configuration modes:

1. **Model-Driven (MD)**: Modern YANG-based configuration (default for SR OS 25+)
2. **Classic CLI**: Traditional CLI configuration mode

The configuration mode is determined by the `SROS_CONFIG_MODE` environment variable and can be:
- `model-driven` (default)
- `classic`
- `mixed` (treated as classic)

**Note**: SAR-Hm nodes only support classic mode and will be automatically switched if configured otherwise.

### Template Selection Logic

Configuration templates are selected based on:

```
┌─────────────────────────────────────────┐
│   Configuration Mode Check              │
└─────────────┬───────────────────────────┘
              │
              ├─ Classic/Mixed Mode
              │  ├─ IXR → cfgTplClassicIxr
              │  ├─ SAR → cfgTplClassicSar
              │  └─ Other → cfgTplClassic
              │
              └─ Model-Driven Mode
                 └─ All → cfgTplSROS25
```

### Node-Type Specific Configurations

Different platform types have specific configuration overrides:

| Platform | System Config | gRPC Config | Special Notes |
|----------|--------------|-------------|---------------|
| IXR | `systemCfgIXR` | `grpcConfigIXR` | Secure/Insecure variants |
| SAR | `systemCfgSAR` | `grpcConfigSAR` | Secure/Insecure variants |
| SAR-Hm | `systemCfgSAR` | N/A | Classic mode only, no gRPC |
| SR/VSR | `systemCfg` | `grpcConfig` | Default configs |

### Version Detection

The implementation can detect SR OS versions through multiple methods:

1. **Image Labels**: Checks for `sros.version` label in container image
2. **Graph Driver**: Reads `/etc/sros-version` directly from overlay2 filesystem layers
3. **Running Container**: Executes `cat /etc/sros-version` in running container

This is done **before** configuration generation to support version-specific templates in the future.

```go
// Version detection flow
getImageSrosVersion() 
  └─> Check image labels
      └─> Read from GraphDriver.Data.UpperDir
          └─> Fallback to layer traversal
              └─> Default to v25.0.0
```

### Template Data Structure

The `srosTemplateData` struct consolidates all information needed for both template selection and execution:

```go
type srosTemplateData struct {
    // Selection criteria
    NodeType          string
    ConfigurationMode string
    SwVersion         *SrosVersion
    IsSecureGrpc      bool
    
    // Certificate data
    TLSKey, TLSCert, TLSAnchor string
    
    // Network configuration
    IFaces      map[string]tplIFace
    DNSServers  []string
    
    // Service configurations
    SystemConfig, GRPCConfig, NetconfConfig string
    // ... and more
}
```

### Distributed SR OS Support

The implementation supports distributed SR OS deployments with multiple components:

- **Base Node**: Orchestrates the deployment
- **CPM (Control Processing Module)**: Control plane (slots A, B)
- **IOM/Linecard**: Data plane (numbered slots)

Configuration generation handles distributed setups by:
1. Generating component-specific configuration
2. Coordinating deployment order (LCs first, then CPMs)
3. Managing component-specific environment variables

## Configuration Flow

```
createSROSConfigFiles()
  │
  ├─> getImageSrosVersion()           # Detect SR OS version
  │
  ├─> prepareConfigTemplateData()     # Prepare template data
  │   ├─> Gather node information
  │   ├─> Load certificates
  │   ├─> Prepare SSH keys
  │   └─> applyNodeTypeSpecificConfig()  # Apply platform-specific configs
  │
  ├─> selectConfigTemplate()          # Choose appropriate template
  │   └─> Returns template based on mode and platform
  │
  └─> addDefaultConfig()              # Generate and apply config
      └─> Execute template with prepared data
```

## Key Features

### Security

- **TLS Certificate Management**: Automatic certificate generation and injection
- **Secure gRPC**: Support for both secure and insecure gRPC configurations
- **SSH Key Injection**: Automatic SSH public key configuration from host

### Networking

- **Management Interface**: Automatic IP assignment and configuration
- **Service Discovery**: `/etc/hosts` population for inter-node communication
- **MTU Configuration**: Configurable MTU for management interfaces
- **TX Offload Disable**: Automatic disabling of TX checksum offload on host veth

### Monitoring

- **Health Checks**: Container health monitoring during PostDeploy
- **Log Monitoring**: Real-time log scanning for errors during boot
- **Ready Detection**: PID-based readiness detection

### Configuration Management

- **Partial Config Support**: Files with `.partial` suffix are merged with defaults
- **Full Config Support**: Complete startup configurations bypass default generation
- **Component Config Generation**: Automatic configuration for distributed deployments
- **Auto-save**: Configuration automatically saved to startup on boot

## Environment Variables

Key environment variables used by SR OS nodes:

- `SROS_CONFIG_MODE`: Configuration mode (model-driven/classic/mixed)
- `NOKIA_SROS_SLOT`: Slot identifier for distributed deployments
- `NOKIA_SROS_NUM_OF_CPMS`: Number of CPMs in distributed setup

## File Organization

```
sros/
├── sros.go                    # Main node implementation
├── sros_config.go            # Configuration generation
├── version.go                # Version detection and parsing
├── sros_test.go             # Unit tests
├── configs/                  # Embedded configuration templates
│   ├── sros_config_sros25.go.tpl      # Model-driven template
│   ├── sros_config_classic.go.tpl     # Classic CLI template
│   ├── ixr/                           # IXR-specific configs
│   │   ├── 12_grpc.cfg
│   │   ├── 12_grpc_insecure.cfg
│   │   └── 14_system.cfg
│   └── sar/                           # SAR-specific configs
│       ├── 12_grpc.cfg
│       ├── 12_grpc_insecure.cfg
│       └── 14_system.cfg
└── README.md                 # This file
```

## Testing

Run unit tests with:

```bash
# Run all tests
go test -v

# Run specific test
go test -v -run TestNodeTypeHelpers

# Run with coverage
go test -cover
```

## Future Enhancements

### Version-Specific Templates

The infrastructure is in place to support different configuration templates based on SR OS version:

```go
// Future implementation
if tplData.SwVersion.Major >= 26 {
    tmpl = cfgTplSROS26
    tplName = "clab-sros-config-sros26"
}
```

### Additional Platform Support

The modular design allows easy addition of new platform types by:
1. Adding new regexp pattern
2. Creating platform-specific helper method
3. Adding platform-specific configuration files
4. Updating `applyNodeTypeSpecificConfig()` logic

## References

- [Containerlab Documentation](https://containerlab.dev/)
- [Nokia SR OS Documentation](https://documentation.nokia.com/)
- [Docker Overlay2 Storage Driver](https://docs.docker.com/storage/storagedriver/overlayfs-driver/)

## Contributing

When making changes to this implementation:

1. **Add tests** for new functionality
2. **Update helper methods** if adding new node type detection
3. **Document** configuration template changes
4. **Run linting**: `golangci-lint run`
5. **Format code**: `gofmt -w *.go`
6. **Verify tests pass**: `go test -v`

## Troubleshooting

### Version Detection Fails

If version detection fails, check:
- Docker daemon is accessible
- Container image has `/etc/sros-version` file
- Permission to access `/var/lib/docker` (for graph driver method)

### Configuration Not Applied

Check:
- Configuration mode is correctly set in environment variables
- Node type is correctly specified
- Startup config file format (partial vs. full)
- Container logs: `docker logs -f <container-name>`

### Distributed Deployment Issues

Verify:
- Component slots are correctly configured
- CPM slots are specified (A, B, or both)
- Linecard slots are numeric
- Environment variables are set correctly
- CPM slots are specified (A, B, or both)
- Linecard slots are numeric
- Environment variables are set correctly