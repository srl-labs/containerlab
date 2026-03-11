# SR OS Node Implementation for Containerlab

This directory contains the implementation of Nokia SR OS (Service Router Operating System) nodes for Containerlab.

## Overview

The SR OS node kind supports deploying Nokia's SR OS network operating system in containerized environments. This implementation handles various SR OS platforms including:

- **SR routers** (sr-, vsr- series) - Traditional service routers and virtual service routers
- **IXR routers** (ixr- series) - IP/MPLS routers for datacenter interconnect and edge applications
- **SAR routers** (sar- series, including sar-hm variants) - Service Aggregation Routers

## Key Components

### Core Files

- **`sros.go`**: Main implementation of the SR OS node kind
  - Node lifecycle management (Init, PreDeploy, Deploy, PostDeploy, Delete)
  - Container configuration and management
  - Distributed fabric deployment support
  - Network interface configuration
  - Helper methods for node type detection
  - Template-based configuration generation
  - Support for both Model-Driven (MD) and Classic CLI modes
  - Node-type specific configuration (IXR, SAR, SR)
  - TLS certificate management
  - gRPC, NETCONF, SSH configuration
  - Template selection logic
  - Configuration rendering pipeline

- **`version.go`**: SR OS version detection and parsing
  - Extracts version from running containers
  - Reads version from image layers without spawning containers
  - Parses various version string formats (e.g., "25.10.R1", "0.0.I8306")
  - Version comparison and string representation

- **`sros_test.go`**: Unit tests
  - Node type helper method tests
  - Edge case validation
  - Version parsing tests

- During PreDeploy, `chassis_info.json` is read from the node image (`/opt/nokia/chassis_info.json`) and written under the node's lab directory (`n.Cfg.LabDir`) with the same filename.

## Architecture

### Node Type Detection

The implementation uses helper methods to identify different SR OS platforms with case-insensitive matching:

```go
// Helper methods in sros.go
isIXRNode()    // Matches: ixr-, IXR-, Ixr- (case-insensitive)
isSARNode()    // Matches: sar-, SAR-, Sar- (case-insensitive)  
isSARHmNode()  // Matches: sar-hm, sar-h, sar-m, SAR-HM, SAR-H, SAR-M (case-insensitive)
```

#### Regular Expression Patterns

These helpers use pre-compiled regular expressions defined at package level for optimal performance:

```go
var (
    // (?i) = case-insensitive flag
    // \b = word boundary (ensures full word match, not substring)
    sarHmRegexp = regexp.MustCompile(`(?i)\b(sar-hm|sar-h|sar-m|sar-hmc)\b`)
    sarRegexp   = regexp.MustCompile(`(?i)\bsar-`)
    ixrRegexp   = regexp.MustCompile(`(?i)\bixr-`)
)
```

**Why word boundaries matter:**
- `"sar-8"` → matches (has word boundary before 'sar-')
- `"mysarnode"` → doesn't match (no word boundary)
- `"SAR-HM"` → matches (case-insensitive)
- `"sar-a"` → matches SAR but not SAR-Hm

### Configuration Modes

SR OS supports two primary configuration modes that affect the CLI syntax and configuration structure:

#### 1. Model-Driven (MD) Configuration

**Default mode for SR OS 25+**

- YANG-based configuration model
- Structured, hierarchical configuration
- Better validation and error reporting
- JSON/XML API support
- Configuration stored in data model format

**Example MD Configuration:**
```
/configure {
    system {
        name "router1"
        management-interface {
            cli {
                md-cli {
                    auto-config-save true
                }
            }
        }
    }
}
```

#### 2. Classic CLI Configuration

**Traditional SR OS configuration mode**

- Legacy CLI syntax
- Flat command structure with contexts
- Backwards compatible with older SR OS versions
- Configuration stored as CLI commands

**Example Classic Configuration:**
```
configure
    system
        name "router1"
    exit
    management-interface
        cli
            classic-cli
                auto-config-save
            exit
        exit
    exit
exit
```

#### Configuration Mode Detection

The mode is determined by the `SROS_CONFIG_MODE` environment variable:

```go
// Environment variable values
"model-driven"  → Model-Driven mode (default)
"classic"       → Classic CLI mode
"mixed"         → Treated as Classic mode
```

**Special Case: SAR-Hm Nodes**

SAR-Hm nodes **only support Classic mode**. The implementation automatically overrides the configuration mode:

```go
if n.isSARHmNode() {
    if tplData.ConfigurationMode != "classic" {
        log.Warn("SAR-Hm nodes only support classic configuration mode. Overriding...")
        tplData.ConfigurationMode = "classic"
        n.Cfg.Env[envSrosConfigMode] = "classic"
    }
}
```

## Template Selection and Configuration Rendering Process

### High-Level Overview

The configuration generation process follows a sophisticated pipeline that adapts to different node types, configuration modes, and security requirements. Here's the complete flow:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Configuration Generation Flow                 │
└─────────────────────────────────────────────────────────────────┘

1. createSROSConfigFiles()
   │
   ├─> Version Detection Phase
   │   └─> getImageSrosVersion()
   │       ├─> Check image labels (sros.version)
   │       ├─> Read from GraphDriver.Data.UpperDir
   │       ├─> Fallback to layer traversal
   │       └─> Store in n.swVersion
   │
   ├─> Startup config build phase
   │   └─> buildStartupConfig()
   │       ├─> If full user config: read file, substitute, return
   │       └─> Else: addDefaultConfig() then addPartialConfig()
   │           └─> addDefaultConfig()
   │               ├─> prepareConfigTemplateData() (variant → snippet set)
   │               ├─> selectConfigTemplate() (variant → template)
   │               └─> Execute template → append to startup config
   │       └─> Return full startup config string
   │
   └─> Write config to node
       └─> GenerateConfig(dst, startupConfig)

2. Store the config file on the node filesystem
2. PostDeploy Phase
   └─> Apply extra configuration to running container that could not be applied during bootime
```

### Detailed Process Breakdown

#### Phase 1: Version Detection (`getImageSrosVersion`)

Before any configuration is generated, the system determines the SR OS version to support version-specific templates in the future.

```go
func (n *sros) getImageSrosVersion(ctx context.Context) (*SrosVersion, error)
```

**Method 1: Image Labels (Fastest)**
```
InspectImage(image) → Check imageInspect.Config.Labels["sros.version"]
```
- Checks if the container image has a `sros.version` label
- Requires images to be built with version labels

**Method 2: Graph Driver Access (Fast)**
```
InspectImage(image) → Read from imageInspect.GraphDriver.Data.UpperDir
                   └─> /var/lib/docker/overlay2/<hash>/diff/etc/sros-version
```
- Direct filesystem access to image layers
- Uses Docker's overlay2 storage driver
- No container spawning required
- Reads `/etc/sros-version` from the topmost layer

**Method 3: Layer Traversal (Fallback)**
```
InspectImage(image) → Iterate through imageInspect.RootFS.Layers[]
                   └─> Check each layer for etc/sros-version file
```
- Searches through all image layers
- Used when GraphDriver data is unavailable
- More reliable but slightly slower

**Method 4: Default Version**
```
Return &SrosVersion{Major: "25", Minor: "0", Build: "0"}
```
- Used when all other methods fail
- Assumes SR OS 25+

**Version String Parsing**

The parser handles multiple version formats:

```go
// Supported formats:
"v25.10.R1"     → SrosVersion{Major: "25", Minor: "10", Build: "R1"}
"25.10.R1"      → SrosVersion{Major: "25", Minor: "10", Build: "R1"}
"0.0.I8306"     → SrosVersion{Major: "0", Minor: "0", Build: "I8306"}
"24.7.R2"       → SrosVersion{Major: "24", Minor: "7", Build: "R2"}

// Regex pattern: v?(\d+)\.(\d+)\.([A-Za-z0-9]+)
// - v? = optional 'v' prefix
// - (\d+) = major version (digits)
// - \. = literal dot
// - (\d+) = minor version (digits)
// - \. = literal dot
// - ([A-Za-z0-9]+) = build/release (alphanumeric)
```

#### Phase 2: Template Data Preparation (`prepareConfigTemplateData`)

This phase gathers all information needed for configuration generation into a single consolidated structure.

```go
func (n *sros) prepareConfigTemplateData() (*srosTemplateData, error)
```

**Step 2.1: Initialize Base Template Data**

```go
tplData := &srosTemplateData{
    // Selection criteria (used by selectConfigTemplate)
    NodeType:          strings.ToLower(n.Cfg.NodeType),  // "ixr-6e", "sar-8", etc.
    ConfigurationMode: strings.ToLower(n.Cfg.Env[envSrosConfigMode]),  // "model-driven", "classic"
    SwVersion:         n.swVersion,                      // Detected version
    IsSecureGrpc:      *n.Cfg.Certificate.Issue,        // true/false for TLS
    
    // Node identification
    Name:              n.Cfg.ShortName,                  // "router1"
    
    // Certificate data (if TLS enabled)
    TLSKey:            n.Cfg.TLSKey,                     // PEM-encoded private key
    TLSCert:           n.Cfg.TLSCert,                    // PEM-encoded certificate
    TLSAnchor:         n.Cfg.TLSAnchor,                  // PEM-encoded CA certificate
    
    // Banner and authentication
    Banner:            b,                                 // Generated from banner()
    IFaces:            map[string]tplIFace{},            // Interface configurations
    MgmtMTU:           0,
    MgmtIPMTU:         n.Runtime.Mgmt().MTU,             // Management interface MTU
    ComponentConfig:   componentConfig,                   // For distributed setups
    
    // Default service configurations (will be overridden based on node type)
    SystemConfig:      systemCfg,        // Embedded from configs/14_system.cfg
    SNMPConfig:        snmpv2Config,     // Embedded from configs/10_snmpv2.cfg
    GRPCConfig:        grpcConfig,       // Embedded from configs/12_grpc.cfg
    NetconfConfig:     netconfConfig,    // Embedded from configs/13_netconf.cfg
    LoggingConfig:     loggingConfig,    // Embedded from configs/11_logging.cfg
    SSHConfig:         sshConfig,        // Embedded from configs/15_ssh.cfg
}
```

**What are these embedded configs?**

Each service configuration is a string embedded at compile time using Go's `//go:embed` directive:

```go
//go:embed configs/12_grpc.cfg
grpcConfig string

// configs/12_grpc.cfg contains:
configure {
    system {
        grpc {
            admin-state enable
            allow-unsecure-connection
            gnmi {
                admin-state enable
                auto-config-save true
            }
        }
    }
}
```

**Step 2.2: DNS Configuration**

```go
if n.Config().DNS != nil {
    tplData.DNSServers = append(tplData.DNSServers, n.Config().DNS.Servers...)
}
```

**Step 2.3: SSH Public Key Preparation**

```go
n.prepareSSHPubKeys(tplData)
```

This method:
1. Reads SSH public keys from the host system (`~/.ssh/id_*.pub`)
2. Parses them into SSH public key format
3. Adds them to `tplData.SSHPubKeys` for injection into SR OS

**Step 2.4: Component Configuration Generation (Distributed Setups)**

```go
componentConfig := ""
if !isFullConfigFile(n.Cfg.StartupConfig) {
    componentConfig = n.generateComponentConfig()
}
```

For distributed SR OS deployments with multiple CPMs and linecards, this generates:
- Slot-specific configuration
- Card type definitions
- MDA (Media Dependent Adapter) configurations
- Fabric connectivity

Example generated component config:
```
/configure {
    card 1 {
        card-type iom-e
        mda 1 {
            mda-type me12-100gb-qsfp28
        }
    }
    card A {
        card-type cpm-ixr
    }
}
```

#### Phase 3: Node-Type Specific Configuration (`applyNodeTypeSpecificConfig`)

This phase customizes the template data based on the node's platform type and security settings.

```go
func (n *sros) applyNodeTypeSpecificConfig(tplData *srosTemplateData)
```

**Decision Tree:**

```
Start
  │
  ├─> Is IXR Node? (n.isIXRNode())
  │   YES │
  │       ├─> Set SystemConfig = systemCfgIXR
  │       │   • IXR-specific system settings
  │       │   • Different process management
  │       │   • Platform-specific limits
  │       │
  │       ├─> Is Secure gRPC?
  │       │   YES: GRPCConfig = grpcConfigIXR
  │       │   NO:  GRPCConfig = grpcConfigIXRInsecure
  │       │
  │       └─> RETURN (skip SAR checks)
  │
  ├─> Is SAR Node? (n.isSARNode())
  │   YES │
  │       ├─> Set SystemConfig = systemCfgSAR
  │       │   • SAR-specific system settings
  │       │   • Service aggregation features
  │       │   • Platform-specific capabilities
  │       │
  │       ├─> Is Secure gRPC?
  │       │   YES: GRPCConfig = grpcConfigSAR
  │       │   NO:  GRPCConfig = grpcConfigSARInsecure
  │       │
  │       ├─> Is SAR-Hm Node? (n.isSARHmNode())
  │       │   YES │
  │       │       ├─> Force Classic Mode
  │       │       │   • SAR-Hm doesn't support MD mode
  │       │       │   • Override ConfigurationMode
  │       │       │   • Update environment variable
  │       │       │
  │       │       └─> No gRPC support
  │       │           • SAR-Hm classic mode excludes gRPC
  │       │
  │       └─> RETURN
  │
  └─> Default SR/VSR Node
      │
      └─> Is Secure gRPC?
          YES: Keep GRPCConfig = grpcConfig
          NO:  Set GRPCConfig = grpcConfigInsecure
```

**Configuration Override Examples:**

**IXR Node with Secure gRPC:**
```go
tplData.SystemConfig = systemCfgIXR
tplData.GRPCConfig = grpcConfigIXR
// Contains IXR-specific settings:
// - CPM redundancy settings
// - IXR platform limits
// - gRPC with TLS authentication
```

**SAR-Hm Node:**
```go
tplData.SystemConfig = systemCfgSAR
tplData.ConfigurationMode = "classic"
// No GRPCConfig - SAR-Hm classic doesn't support gRPC
// Force classic mode regardless of user setting
```

**SR Node with Insecure gRPC:**
```go
tplData.SystemConfig = systemCfg
tplData.GRPCConfig = grpcConfigInsecure
// Standard SR system config
// gRPC without TLS (for lab/testing)
```

#### Phase 4: Template Selection (`selectConfigTemplate`)

This is where the actual Go template is chosen based on the prepared data.

```go
func (n *sros) selectConfigTemplate(tplData *srosTemplateData) (*template.Template, error)
```

**Selection Algorithm:**

```
┌────────────────────────────────────────────────────────────────┐
│                     Template Selection Logic                    │
└────────────────────────────────────────────────────────────────┘

Check: ConfigurationMode
  │
  ├─> Is "classic" OR "mixed"?
  │   YES │
  │       ├─> Check Node Type
  │       │   │
  │       │   ├─> Is IXR Node?
  │       │   │   YES: Template = cfgTplClassicIxr
  │       │   │        Name = "clab-sros-config-classic-ixr"
  │       │   │        • IXR-specific classic CLI template
  │       │   │        • IXR system commands
  │       │   │        • No gRPC in classic for IXR
  │       │   │
  │       │   ├─> Is SAR Node?
  │       │   │   YES: Template = cfgTplClassicSar
  │       │   │        Name = "clab-sros-config-classic-sar"
  │       │   │        • SAR-specific classic CLI template
  │       │   │        • SAR system commands
  │       │   │        • No gRPC in classic for SAR
  │       │   │
  │       │   └─> Otherwise (SR/VSR)
  │       │       Template = cfgTplClassic
  │       │       Name = "clab-sros-config-classic"
  │       │       • Standard classic CLI template
  │       │       • Full gRPC support
  │       │       • NETCONF support
  │       │
  │       └─> Parse Template with Custom Functions
  │
  └─> Is "model-driven"? (DEFAULT)
      YES │
          ├─> Template = cfgTplSROS25
          │   Name = "clab-sros-config-sros25"
          │   • Model-Driven configuration syntax
          │   • Works for all node types (IXR, SAR, SR)
          │   • Full gRPC/NETCONF support
          │   • Future: version-specific templates
          │
          └─> Parse Template with Custom Functions
```

**Template Variables Matrix:**

| Node Type | Config Mode | Template Used | Features |
|-----------|-------------|---------------|----------|
| SR/VSR | Model-Driven | `cfgTplSROS25` | Full MD config, gRPC, NETCONF |
| SR/VSR | Classic | `cfgTplClassic` | Classic CLI, gRPC, NETCONF |
| IXR | Model-Driven | `cfgTplSROS25` | Full MD config, gRPC (no RIB), NETCONF |
| IXR | Classic | `cfgTplClassicIxr` | IXR classic CLI, gRPC (no RIB), NETCONF |
| SAR | Model-Driven | `cfgTplSROS25` | Full MD config, gRPC (no RIB), NETCONF |
| SAR | Classic | `cfgTplClassicSar` | SAR classic CLI, no gRPC |
| SAR-Hm | Classic (forced) | `cfgTplClassicSar` | SAR classic CLI, no gRPC |

**Custom Template Functions:**

Templates are parsed with custom utility functions provided by Containerlab:

```go
template.New(tplName).
    Funcs(clabutils.CreateFuncs()).  // Custom functions for templates
    Parse(tmpl)
```

These functions include:
- String manipulation (trim, replace, etc.)
- Network utilities (IP address formatting)
- Encoding functions (base64, etc.)
- Conditional logic helpers

#### Phase 5: Template Rendering (`addDefaultConfig`)

This is the final phase where the selected template is executed with the prepared data to generate the actual configuration.

```go
func (n *sros) addDefaultConfig() error
```

**Step 5.1: Orchestration**

```go
// 1. Prepare all template data
tplData, err := n.prepareConfigTemplateData()
if err != nil {
    return err
}

// 2. Select appropriate template
srosCfgTpl, err := n.selectConfigTemplate(tplData)
if err != nil {
    return fmt.Errorf("failed to select config template: %w", err)
}

// 3. Log the selection
log.Debugf("Prepare %q config for %q using template %q", 
    tplData.ConfigurationMode, n.Cfg.LongName, srosCfgTpl.Name())
```

**Step 5.2: Template Execution**

```go
// Execute template with prepared data
buf := new(bytes.Buffer)
err = srosCfgTpl.Execute(buf, tplData)
if err != nil {
    return err
}
```

The Go template engine processes the template string, replacing placeholders with actual values from `tplData`.

**Template Example (Model-Driven):**

Input template (`cfgTplSROS25`):
```go
/configure {
    system {
        name "{{ .Name }}"
        {{- if .Banner }}
        login-banner {
            message "{{ .Banner }}"
        }
        {{- end }}
        {{- if .DNSServers }}
        dns {
            {{- range .DNSServers }}
            address-pref ipv4-only
            name-server "{{ . }}"
            {{- end }}
        }
        {{- end }}
    }
    {{ .SystemConfig }}
    {{ .GRPCConfig }}
    {{ .NetconfConfig }}
    {{- if .SSHPubKeys }}
    system {
        security {
            user-params {
                local-user {
                    user "admin" {
                        {{- range .SSHPubKeys }}
                        public-keys {
                            rsa {
                                key "{{ .Type }} {{ .Key }}"
                            }
                        }
                        {{- end }}
                    }
                }
            }
        }
    }
    {{- end }}
}
```

Rendered output (with sample data):
```
/configure {
    system {
        name "router1"
        login-banner {
            message "Welcome to SR OS Router"
        }
        dns {
            address-pref ipv4-only
            name-server "8.8.8.8"
            name-server "1.1.1.1"
        }
    }
    configure {
        system {
            management-interface {
                cli {
                    md-cli {
                        auto-config-save true
                    }
                }
            }
        }
    }
    configure {
        system {
            grpc {
                admin-state enable
                gnmi {
                    admin-state enable
                    auto-config-save true
                }
            }
        }
    }
    system {
        security {
            user-params {
                local-user {
                    user "admin" {
                        public-keys {
                            rsa {
                                key "ssh-rsa AAAAB3NzaC1yc2EAAAA..."
                            }
                        }
                    }
                }
            }
        }
    }
}
```

**Step 5.3: Configuration Storage**

```go
if buf.Len() == 0 {
    log.Warn("Buffer empty, template parsing error",
        "node", n.Cfg.ShortName,
        "template", srosCfgTpl.Name())
} else {
    log.Debug("Additional default config parsed", 
        "node", n.Cfg.ShortName, 
        "template", srosCfgTpl.Name())
    
    // Append generated config to existing startup config
    n.startupCliCfg = append(n.startupCliCfg, buf.String()...)
}
```

The generated configuration is:
1. Stored in `n.startupCliCfg` (byte slice)
2. Later written to disk at: `<lab-dir>/<node-name>/A/config/cf3/config.cfg`
3. Mounted into the container at boot time
4. Applied by SR OS during initialization

### Complete Configuration Generation Example

Let's walk through a complete example for an **IXR-6e router with secure gRPC in Model-Driven mode**:

```yaml
# Containerlab topology file
nodes:
  ixr1:
    kind: nokia_srsim
    type: ixr-6e
    image: registry.srlinux.dev/pub/sros:25.10.R1
    env:
      SROS_CONFIG_MODE: model-driven
    startup-config: configs/ixr1.partial.cfg
    certificate:
      issue: true
```

**Phase-by-Phase Execution:**

1. **Version Detection:**
   ```
   → getImageSrosVersion()
   → Reads from image GraphDriver
   → Returns: SrosVersion{Major: "25", Minor: "10", Build: "R1"}
   ```

2. **Template Data Preparation:**
   ```go
   tplData = {
       NodeType: "ixr-6e",
       ConfigurationMode: "model-driven",
       SwVersion: {Major: "25", Minor: "10", Build: "R1"},
       IsSecureGrpc: true,
       Name: "ixr1",
       TLSKey: "-----BEGIN PRIVATE KEY-----...",
       TLSCert: "-----BEGIN CERTIFICATE-----...",
       DNSServers: ["8.8.8.8"],
       SystemConfig: systemCfg,
       GRPCConfig: grpcConfig,
       // ... more fields
   }
   ```

3. **Node-Type Config Application:**
   ```
   → isIXRNode() = true
   → tplData.SystemConfig = systemCfgIXR
   → tplData.GRPCConfig = grpcConfigIXR (secure)
   ```

4. **Template Selection:**
   ```
   → ConfigurationMode = "model-driven"
   → Selected: cfgTplSROS25
   → Template name: "clab-sros-config-sros25"
   ```

5. **Template Rendering:**
   ```
   → Execute template with tplData
   → Generate ~500 lines of MD configuration
   → Includes: system config, IXR settings, secure gRPC, TLS certs
   → Store in n.startupCliCfg
   ```

6. **File Writing:**
   ```
   → Write to: clab-ixr-lab/ixr1/tftpboot/config.txt
   → Container mounts this at boot
   → SR OS applies configuration automatically
   ```

### Configuration Merge Strategy

The implementation supports three configuration strategies:

#### 1. Full Configuration (No Merge)

**Trigger:** Startup config file exists and does NOT contain `.partial` in filename

```go
isFullConfigFile(n.Cfg.StartupConfig)  // returns true
```

**Behavior:**
- User-provided configuration is used as-is
- No default configuration is generated
- No template processing occurs
- System skips `addDefaultConfig()` entirely

**Use Case:**
- Complete, production-ready configurations
- Migrated configurations from existing routers
- Configurations with custom, non-standard settings

**Example:**
```yaml
nodes:
  router1:
    startup-config: full-config.txt  # Complete config, no .partial
```

#### 2. Partial Configuration (Merge with Defaults)

**Trigger:** Startup config file exists and CONTAINS `.partial` in filename

```go
isPartialConfigFile(n.Cfg.StartupConfig)  // returns true
```

**Behavior:**
- User configuration is loaded first
- Default configuration is generated via templates
- Both configurations are merged:
  ```
  Final Config = User Partial Config + Generated Defaults
  ```

**Merge Order:**
```
1. User's partial config (read from file)
2. Generated system config (from systemCfg)
3. Generated gRPC config (from grpcConfig)
4. Generated NETCONF config (from netconfConfig)
5. Generated SSH keys (from host)
6. Generated TLS certificates (if enabled)
```

**Use Case:**
- Quick deployments with minimal custom config
- Testing scenarios
- Labs where most settings can be default

**Example:**
```yaml
nodes:
  router1:
    startup-config: custom-interfaces.partial.cfg  # Partial config
```

**Sample Partial Config File:**
```
# custom-interfaces.partial.cfg
/configure {
    port 1/1/c1 {
        admin-state enable
        connector {
            breakout c1-100g
        }
    }
}
```

**Resulting Merged Config:**
```
# User's partial config (loaded first)
/configure {
    port 1/1/c1 {
        admin-state enable
        connector {
            breakout c1-100g
        }
    }
}

# Generated defaults (appended)
/configure {
    system {
        name "router1"
        login-banner {
            message "Containerlab Node"
        }
        grpc {
            admin-state enable
            gnmi {
                admin-state enable
            }
        }
    }
}
```

#### 3. No Configuration (Defaults Only)

**Trigger:** No startup config file specified

```go
n.Cfg.StartupConfig == ""  // true
```

**Behavior:**
- Only generated default configuration is used
- Full template processing occurs
- All defaults from embedded configs applied

**Use Case:**
- Quick proof-of-concept setups
- Learning/training labs
- Automated testing

**Example:**
```yaml
nodes:
  router1:
    kind: nokia_srsim
    # No startup-config specified
```

### Node-Type Specific Configurations

Each platform type has tailored configurations embedded in the `configs/` directory.

#### System Configurations

**Standard SR/VSR (`configs/14_system.cfg`):**
```
/configure {
    system {
        management-interface {
            configuration-mode model-driven
            cli {
                md-cli {
                    auto-config-save true
                    environment {
                        command-alias {
                            alias "info" {
                                admin-state enable
                                python-script "show_version"
                            }
                        }
                    }
                }
            }
        }
    }
}
```

**IXR Nodes (`configs/ixr/14_system.cfg`):**
```
/configure {
    system {
        management-interface {
            configuration-mode model-driven
            cli {
                md-cli {
                    auto-config-save true
                }
            }
        }
        grpc {
            admin-state enable
        }
    }
}
```
**Key Differences:**
- IXR excludes RIB API config
- Different process management settings
- Platform-specific resource limits

**SAR Nodes (`configs/sar/14_system.cfg`):**
```
/configure {
    system {
        management-interface {
            configuration-mode model-driven
            cli {
                md-cli {
                    auto-config-save true
                }
            }
        }
    }
}
```
**Key Differences:**
- SAR-specific 
- Different card/MDA configuration

#### gRPC Configurations

**Secure gRPC (`configs/12_grpc.cfg`):**
```
/configure {
    system {
        grpc {
            admin-state enable
            tls-server-profile "grpc-tls"
            gnmi {
                admin-state enable
                auto-config-save true
            }
            rib-api {
                admin-state enable
            }
        }
        security {
            tls {
                server-profile "grpc-tls" {
                    trust-anchor-profile "ca-profile"
                    key-file "system-rsa-key"
                    cert-file "system-cert"
                }
            }
        }
    }
}
```

**Insecure gRPC (`configs/12_grpc_insecure.cfg`):**
```
/configure {
    system {
        grpc {
            admin-state enable
            allow-unsecure-connection
            gnmi {
                admin-state enable
                auto-config-save true
            }
        }
    }
}
```
**Key Difference:** `allow-unsecure-connection` allows gRPC without TLS

**IXR Secure gRPC (`configs/ixr/12_grpc.cfg`):**
```
/configure {
    system {
        grpc {
            admin-state enable
            tls-server-profile "grpc-tls"
            gnmi {
                admin-state enable
                auto-config-save true
            }
        }
    }
}
```

**SAR Classic Mode:**
- **No gRPC configuration embedded**
- SAR in classic mode doesn't support gRPC
- Uses only CLI/NETCONF for management

### Distributed SR OS Configuration

For distributed deployments with multiple components (CPMs and line cards), additional configuration is generated.

#### Component Configuration Generation

```go
func (n *sros) generateComponentConfig() string
```

**Generated Configuration Structure:**

```
/configure {
    {{- range .Components }}
    card {{ .Slot }} {
        card-type {{ .Type }}
        {{- if .MDAs }}
        {{- range .MDAs }}
        mda {{ .Slot }} {
            mda-type {{ .Type }}
        }
        {{- end }}
        {{- end }}
    }
    {{- end }}
    
    {{- if .FabricConfig }}
    fabric {
        {{ .FabricConfig }}
    }
    {{- end }}
}
```

**Example Distributed Topology:**

```yaml
nodes:
  chassis1:
    kind: nokia_srsim
    components:
      - slot: A
        type: cpm-ixr
      - slot: B
        type: cpm-ixr
      - slot: 1
        type: iom-ixr
        mda:
          - slot: 1
            type: m20-1gb-sfp-b
```

**Generated Component Config:**

```
/configure {
    card A {
        card-type cpm-ixr
    }
    card B {
        card-type cpm-ixr
    }
    card 1 {
        card-type iom-ixr
        mda 1 {
            mda-type m20-1gb-sfp-b
        }
    }
    fabric {
        multiplane-mode enabled
    }
}
```

### Security and Certificate Management

When `certificate.issue: true` is set, the system generates and injects TLS certificates.

#### Certificate Generation Flow

```
1. Containerlab Certificate Authority (CA)
   └─> Generates CA certificate and key

2. Node Certificate Generation
   ├─> Generate private key for node
   ├─> Create CSR (Certificate Signing Request)
   ├─> Sign with CA
   └─> Store in node configuration

3. Certificate Injection
   ├─> Write TLS key to: <labdir>/.tls/<node-name>/node.key
   ├─> Write TLS cert to: <labdir>/.tls/<node-name>/node.pem
   └─> Write CA cert to: <labdir>/.tls/ca/ca.(key|pem)

4. SR OS Configuration
   └─> Reference certificates in config:
       /configure system security pki import ...
```

#### TLS Configuration Template

```go
// Generated in template
/configure {
    system {
        security {
            pki {
                ca-profile "ca-profile" {
                    admin-state enable
                    cert-file "{{ .TLSAnchor }}"
                }
            }
            tls {
                server-profile "grpc-tls" {
                    admin-state enable
                    trust-anchor-profile "ca-profile"
                    key-file "{{ .TLSKey }}"
                    cert-file "{{ .TLSCert }}"
                }
            }
        }
        grpc {
            tls-server-profile "grpc-tls"
        }
    }
}
```

### Template Debugging and Validation

#### Enable Debug Logging

```bash
# Run containerlab with debug logging
sudo clab deploy -t topology.yml --debug
```

Debug logs show:
- Selected template name
- Configuration mode detected
- Node type detection results
- Template data structure
- Generated configuration snippets

#### Manual Template Testing

You can test template rendering manually:

```go
package main

import (
    "bytes"
    "fmt"
    "text/template"
)

func main() {
    // Sample template
    tmpl := `
    System Name: {{ .Name }}
    Node Type: {{ .NodeType }}
    Mode: {{ .ConfigurationMode }}
    `
    
    // Sample data
    data := map[string]string{
        "Name": "router1",
        "NodeType": "ixr-6e",
        "ConfigurationMode": "model-driven",
    }
    
    // Parse and execute
    t := template.Must(template.New("test").Parse(tmpl))
    buf := new(bytes.Buffer)
    t.Execute(buf, data)
    
    fmt.Println(buf.String())
}
```

#### Configuration Validation

After generation, validate the configuration:

```bash
# SSH into the container
ssh admin@clab-mylab-router1 

# Check configuration
A:admin@router1# admin display-config
```

### Future Enhancements

#### Version-Specific Templates

The infrastructure supports version-based template selection:

```go
// Planned enhancement
func (n *sros) selectConfigTemplate(tplData *srosTemplateData) (*template.Template, error) {
    // Model-driven mode
    if tplData.ConfigurationMode == "model-driven" {
        // Select template based on SR OS version
        if tplData.SwVersion.Major >= "26" {
            return template.New("clab-sros-config-sros26").
                Funcs(clabutils.CreateFuncs()).
                Parse(cfgTplSROS26)
        } else if tplData.SwVersion.Major >= "25" {
            return template.New("clab-sros-config-sros25").
                Funcs(clabutils.CreateFuncs()).
                Parse(cfgTplSROS25)
        }
    }
    // ... rest of logic
}
```

**Use Cases:**
- SR OS 26 might introduce new configuration syntax
- Different feature sets across versions
- Backwards compatibility maintenance

#### Additional Platform Support for Configuration

To add a new platform type (e.g., "DMS"):

1. **Add regexp pattern:**
   ```go
   dmsRegexp = regexp.MustCompile(`(?i)\dms-1\b`)
   ```

2. **Create helper method:**
   ```go
   func (n *sros) isDMSNode() bool {
       return dmsRegexp.MatchString(n.Cfg.NodeType)
   }
   ```

3. **Add configuration files:**
   ```
   configs/dms/12_grpc.cfg
   configs/dms/14_system.cfg
   ```

4. **Update `applyNodeTypeSpecificConfig`:**
   ```go
   if n.isDMSeNode() {
       tplData.SystemConfig = systemCfgDMS
       tplData.GRPCConfig = grpcConfigDMS
   }
   ```

5. **Add template (if needed):**
   ```go
   //go:embed configs/dms_config_classic.go.tpl
   cfgTplClassicDMS string
   ```

6. **Update `selectConfigTemplate`:**
   ```go
   if n.isDMSNode() {
       tmpl = cfgTplClassicDMS
       tplName = "clab-sros-config-classic-dms"
   }
   ```

## File Organization

```
sros/
├── sros.go                           # Main node implementation
│   ├── Node lifecycle methods
│   ├── Helper methods (isIXRNode, isSARNode, etc.)
│   ├── Distributed deployment logic
│   ├── Container management
│   ├── prepareConfigTemplateData()
│   ├── applyNodeTypeSpecificConfig()
│   ├── selectConfigTemplate()
│   ├── addDefaultConfig()
│   └── Template rendering logic
│
├── version.go                        # Version detection
│   ├── getImageSrosVersion()
│   ├── readVersionFromImageLayers()
│   ├── parseSrosVersion()
│   └── RunningVersion()
│
├── sros_test.go                      # Unit tests
│   ├── TestNodeTypeHelpers
│   ├── TestNodeTypeHelpers_EdgeCases
│   └── Version parsing tests
│
├── configs/                          # Embedded configuration templates
│   ├── sros_config_sros25.go.tpl    # Model-Driven template
│   ├── sros_config_classic.go.tpl   # Classic CLI template
│   │
│   ├── 10_snmpv2.cfg               # SNMP configuration
│   ├── 11_logging.cfg              # Logging configuration
│   ├── 12_grpc.cfg                 # Secure gRPC config
│   ├── 12_grpc_insecure.cfg        # Insecure gRPC config
│   ├── 13_netconf.cfg              # NETCONF configuration
│   ├── 14_system.cfg               # System configuration
│   ├── 15_ssh.cfg                  # SSH configuration
│   │
│   ├── ixr/                         # IXR-specific configs
│   │   ├── 12_grpc.cfg
│   │   ├── 12_grpc_insecure.cfg
│   │   ├── 14_system.cfg
│   │   └── ixr_config_classic.go.tpl
│   │
│   └── sar/                         # SAR-specific configs
│       ├── 12_grpc.cfg
│       ├── 12_grpc_insecure.cfg
│       ├── 14_system.cfg
│       └── sar_config_classic.go.tpl
│
└── README.md                         # This file
```

## Testing

### Run Unit Tests

```bash
# Run all tests
cd /home/schavezc/wk/containerlab/nodes/sros
go test -v

# Run specific test
go test -v -run TestNodeTypeHelpers

# Run with coverage
go test -cover

# Generate coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Testing

Test complete configuration generation:

```bash
# Deploy a test topology
sudo clab deploy -t test-topology.yml --debug

# Verify generated configuration
cat clab-test/router1/A/cf3/config.cfg

# Check running configuration
ssh admin@clab-test-router1 "admin display-config"

# Verify gRPC is working
gnmic -a clab-test-router1:57400 --insecure capabilities

# Clean up
sudo clab destroy -t test-topology.yml --cleanup
```

## Troubleshooting

### Template Selection Issues

**Symptom:** Wrong template is selected

**Debug:**
```bash
# Enable debug logging
sudo clab deploy -t topology.yml --debug 2>&1 | grep "template"
```

**Common Causes:**
- `SROS_CONFIG_MODE` environment variable incorrectly set
- Node type not matching regexp patterns (check case sensitivity)
- SAR-Hm node not forcing classic mode

**Solution:**
```yaml
# Explicitly set configuration mode
nodes:
  router1:
    env:
      SROS_CONFIG_MODE: model-driven  # or classic
```

### Version Detection Fails

**Symptom:** "Failed to extract SR OS version from image layers"

**Debug:**
```bash
# Check if image has version file
docker run --rm <image> cat /etc/sros-version

# Check GraphDriver access
sudo ls -la /var/lib/docker/overlay2/
```

**Common Causes:**
- Image doesn't contain `/etc/sros-version`
- Insufficient permissions to access Docker graph driver
- Non-standard Docker root directory

**Solution:**
```go
// Add version label to image during build
LABEL sros.version="25.10.R1"
```

### Configuration Not Applied

**Symptom:** Generated config exists but not applied to SR OS

**Debug:**
```bash
# Check if config file was written
ls -la clab-mylab/router1/A/cf3/config.cfg

# Check container logs for boot errors
docker logs clab-mylab-router1 2>&1 | grep -i error

# Verify file is mounted
docker exec clab-mylab-router1 ls -la clab-mylab/router1/A/
```

**Common Causes:**
- Configuration syntax errors
- File permissions issues
- Container not reading from correct path

**Solution:**
```bash
# Validate configuration manually
ssh admin@clab-mylab-router1
/configure private
# Then paste config 
/commit
#and check for errors
```

### gRPC Not Working

**Symptom:** Cannot connect via gRPC/gNMI

**Debug:**
```bash
# Check if gRPC is enabled in config
ssh admin@clab-mylab-router1  "show system grpc"

# Verify port is listening
docker exec clab-mylab-router1 netstat -tulpn | grep 57400

# Test connection
gnmic -a <router-ip>:57400 --insecure capabilities
```

**Common Causes:**
- Insecure gRPC config not selected when certs disabled
- SAR-Hm node trying to use gRPC (not supported in classic)
- Firewall blocking port 57400

**Solution:**
```yaml
# For insecure gRPC
nodes:
  router1:
    certificate:
      issue: false  # This will select insecure gRPC config
```

### Distributed Deployment Issues

**Symptom:** CPMs or line cards not appearing in configuration

**Debug:**
```bash
# Verify component configuration was generated
cat clab-mylab/chassis1/A/cf3/config.cfg | grep "card"

# Check if all component containers started
docker ps | grep chassis1

# Verify slot assignments
ssh admin@clab-mylab-chassis1-cpm-a "show card state"
```

**Common Causes:**
- Component slots not properly defined
- CPM slot names must be "A" or "B" (uppercase)
- Line card slots must be numeric
- Environment variables not set on components

## Performance Considerations

### Template Execution Performance

| Operation | Time (approx) | Optimization |
|-----------|---------------|--------------|
| Regexp compilation | 0.001ms | Compile once at package init |
| Version detection (labels) | 1-5ms | Fastest method |
| Version detection (GraphDriver) | 5-20ms | Fast, recommended |
| Version detection (layer traverse) | 50-200ms | Fallback only |
| Template parsing | 1-2ms | Cached after first parse |
| Template execution | 5-10ms | Minimal data processing |
| **Total (optimized)** | **~20-40ms** | Per node |

### Memory Usage

- **Template Data Structure**: ~5-10 KB per node
- **Generated Configuration**: ~10-50 KB per node
- **Embedded Configs**: ~100 KB (shared across all nodes)

### Optimization Tips

1. **Use Image Labels**: Add `sros.version` label to images for fastest version detection
2. **Batch Deployments**: Deploy multiple nodes in parallel (Containerlab handles this)
3. **Reuse Templates**: Template compilation is cached by Go
4. **Minimal Partial Configs**: Smaller partial configs = faster merging

## References

- [Containerlab Documentation](https://containerlab.dev/)
- [Nokia SR OS Documentation](https://documentation.nokia.com/)
- [Go Template Documentation](https://pkg.go.dev/text/template)
- [Docker Overlay2 Storage Driver](https://docs.docker.com/storage/storagedriver/overlayfs-driver/)
- [gRPC/gNMI Protocol](https://github.com/openconfig/gnmi)

## Contributing

### Adding New Features

When contributing to this implementation:

1. **Follow the existing pattern:**
   - Use helper methods for node type detection
   - Add tests for new functionality
   - Document configuration changes

2. **Template Changes:**
   - Keep templates in `configs/` directory
   - Use `//go:embed` for embedding
   - Document template variables
   - Test rendering with sample data

3. **Configuration Logic:**
   - Add platform-specific configs to `applyNodeTypeSpecificConfig()`
   - Update template selection in `selectConfigTemplate()`
   - Maintain backwards compatibility

4. **Testing Requirements:**
   - Unit tests for helper methods
   - Integration tests for full deployments
   - Document test scenarios

5. **Code Quality:**
   ```bash
   # Format code
   gofmt -w *.go
   
   # Run linter
   golangci-lint run
   
   # Run tests
   go test -v
   
   # Check coverage
   go test -cover
   ```

### Pull Request Guidelines

- **Title**: Descriptive and concise
- **Description**: Explain the what and why
- **Tests**: Include unit and integration tests
- **Documentation**: Update README.md if adding features
- **Backwards Compatibility**: Don't break existing deployments

## License

BSD 3-Clause License - See LICENSE file for details

---

**Maintainers:**
- Nokia Containerlab Team
- Community Contributors

**Last Updated:** 2025-11-15
**SR OS Versions Supported:**  25.7+, 26.x (planned)
