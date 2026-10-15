# SR OS Node Implementation for Containerlab

This directory contains the implementation of the `nokia_srsim` kind, which deploys Nokia SR OS (SR-SIM) containers.

## Overview

The kind handles these SR OS platform families:

- **SR routers** (sr-, vsr- series) - Traditional service routers and virtual service routers
- **IXR routers** (ixr- series) - IP/MPLS routers for datacenter interconnect and edge applications
- **SAR routers** (sar- series, including sar-hm variants) - Service Aggregation Routers

## Key Components

- **`sros.go`**: Node lifecycle (Init, PreDeploy, Deploy, PostDeploy, Delete), distributed fabric deployment, TLS bootstrap and config save
- **`config.go`**: Startup config generation, template data, version helpers, admin credentials
- **`template.go`**: Embedded templates, `srosTemplateData`, and template selection
- **`variant.go`**: Config variant (mode, family, secure gRPC) and snippet selection; node type helpers
- **`configs_embed.go`**: Embedded config snippets under `configs/`
- **`version.go`**: SR OS version detection from image labels, image layers, or the running container
- **`component.go`**: Component model, slot checks, and environment precedence
- **`component_config.go`**: Generated card, SFM, XIOM, and MDA configuration, including integrated defaults
- **`chassis_info.go`**: Parses `chassis_info.json` and returns advisory compatibility warnings

### Chassis info and component checks

During PreDeploy, `createSROSFiles` reads `/opt/nokia/chassis_info.json` from the node image and writes it to `n.Cfg.LabDir/chassis_info.json`, prefixed with the license header. Failure to read the file is logged and does not stop deployment.

The same file is then compared with the effective component configuration. Aliases such as `SAR-Mx` to `SAR-Hx` are followed. Unsupported slots, CPM/IOM types, SFM, XIOM, direct MDAs, and fixed-hardware mismatches produce warnings only. MDAs nested under an XIOM are not checked, because the file does not say which MDA types a given XIOM accepts.

### Environment precedence

Node `env` overrides per-component `env`, which overrides `type`, `xiom`, `mda`, and `sfm`. When a node environment variable replaces a different component value, containerlab logs that the component value will not be used and names the variable. `NOKIA_SROS_SLOT` still comes from each component in a distributed node, because one node-level slot cannot identify every card.

## Architecture

### Node Type Detection

`variant.go` matches `type` case-insensitively with package-level regular expressions:

```go
sarRegexp   = regexp.MustCompile(`(?i)\bsar-`)
sarHmRegexp = regexp.MustCompile(`(?i)\b(sar-hm|sar-hmc)\b`)
ixrRegexp   = regexp.MustCompile(`(?i)\bixr-`)
```

`resolveConfigVariant` turns the node into a `ConfigVariant` (mode, family `sr`/`ixr`/`sar`, secure gRPC, forced classic). Everything else selects templates and snippets from that variant.

### Configuration Modes

The `config-mode` kind-specific key selects the mode:

```yaml
config-mode: model-driven  # default
config-mode: classic
config-mode: mixed         # rendered with the classic template
```

SAR-Hm nodes only support classic mode. `resolveConfigVariant` sets `ForceClassic`, and `prepareConfigTemplateData` logs a warning and overrides the mode.

### Version Detection

`createSROSConfigFiles` calls `srosVersionFromImage` before rendering:

1. If the image has `org.opencontainers.image.vendor=Nokia`, `org.opencontainers.image.title=srsim` and `org.opencontainers.image.version`, that label is parsed.
2. Otherwise `/etc/sros-version` is read from the image graph driver (`UpperDir`, then `MergedDir`).
3. If both fail, `srosDefaultVersion` (25.7.R1) is used.

`parseVersionString` uses `v?(\d+)\.(\d+)\.([A-Za-z0-9]+)` and returns `0.0.0` when nothing matches. Development images such as `0.0.0-I8646` parse as `0.0.0`, which is treated as the latest release. PostDeploy replaces the version with the running container's `/etc/sros-version`.

## Startup Configuration

```
createSROSConfigFiles()
  ├─ srosVersionFromImage()
  └─ buildStartupConfig()
       ├─ full startup-config: read file, substitute env/template vars, use as-is
       └─ otherwise:
            ├─ addDefaultConfig()
            │    ├─ prepareConfigTemplateData()   variant → snippets, version flags, credentials
            │    ├─ selectConfigTemplate()        variant + version → template
            │    └─ execute template              append to startupCliCfg
            └─ addPartialConfig()                 append *.partial* file (CPM only)
  → GenerateConfig(<labdir>/<slot>/config/cf3/config.cfg)
```

A startup config is "full" when it is set and its name does not contain `.partial`. Full configs skip generated config, component config, and the PostDeploy save.

### Templates

| Mode | Family | Version | Template |
|------|--------|---------|----------|
| Model-driven | any | SR OS <= 26.3 | `configs/sros_config_le_26_3.go.tpl` |
| Model-driven | any | SR OS 26.7, later, or `0.0.0` | `configs/sros_config.go.tpl` |
| Classic / mixed | SR | any | `configs/sros_config_classic.go.tpl` |
| Classic / mixed | IXR | any | `configs/ixr/ixr_config_classic.go.tpl` |
| Classic / mixed | SAR | any | `configs/sar/sar_config_classic.go.tpl` |

Version-dependent content:

- **`tls-aes128-ccm8-sha256`**: rendered only for SR OS <= 26.3 when `certificate.issue` is true. SR OS 26.7 rejects it as obsolete, which fails the whole startup config. Model-driven gets it from the le-26.3 template; classic templates use the `srosLe26_3` template function.
- **gRPC MD-CLI** (`system grpc md-cli` and `md-cli-session permit`): rendered only when `SrosLe25_10` is true (SR OS <= 25.10; obsolete in 26.3).

Templates are parsed with `srosTemplateFuncs`, which adds `srosLe26_3` to `clabutils.CreateFuncs()`.

### Snippets

Model-driven templates include flat MD-CLI snippets from `configs/`, chosen by `getFullSnippetSet`:

- `10_snmpv2.cfg`, `11_logging.cfg`, `13_netconf.cfg`, `15_ssh.cfg`: shared by all families
- `12_grpc.cfg` / `12_grpc_insecure.cfg`: secure or insecure gRPC, with `ixr/` and `sar/` variants
- `14_system.cfg`: system and AAA profiles, with `ixr/` and `sar/` variants

### Admin Credentials

The admin user and password come from `credentials` (default `admin` / `NokiaSros1!`), are CLI-quoted by `cliQuoted`, and are rendered by every template. PostDeploy SSH and NETCONF use the same credentials.

### Component configuration

`generateComponentConfig` runs for integrated nodes and for the CPM of a distributed node, unless the user supplied a full startup config, selected classic mode, or set `gen-component-config: false`.

Known integrated types (`sr-1`, `sr-1s`, `ixr-r6`, and the `ixr-e*` pizza boxes) get built-in card and MDA lines. One `components` entry is accepted, with the slot omitted or set to `A`. `ixr-r6` also accepts `B`, and two entries (`A` and `B`) deploy a dual-CPIOM node. A component `type` sets `NOKIA_SROS_CARD` and skips generated card and MDA lines.

Distributed line cards produce one line per card, SFM, XIOM, and direct MDA. The values are the effective ones after environment precedence, so `NOKIA_SROS_MDA_*` and `NOKIA_SROS_XIOM_*` change the generated lines.

```
/configure card 1 card-type xcm-2s admin-state enable
/configure card 1 xiom x1 xiom-type iom-s-3.0t admin-state enable
/configure card 1 xiom x1 mda 1 mda-type ms18-100gb-qsfp28 admin-state enable
```

## PostDeploy

On the active CPM, once NETCONF (TCP 830) is reachable:

1. If `certificate.issue` is true, `tlsCertBootstrap` imports the node key and certificate written to `config/cf3` and enables the `clab-grpc-certs` cert-profile.
2. Unless a full startup config was used, `saveConfigWithAddr` saves the running config (NETCONF in model-driven mode, SSH in classic mode).

If the boot log reports `Configuration load failed - using default configuration`, the startup config was rejected and postdeploy actions are skipped. The factory `admin`/`admin` login is then active instead of the configured credentials.

## Testing

```bash
go test ./nodes/sros/ -count=1
```

`admin_credentials_test.go` renders the templates and checks credentials and the version-dependent lines. Integration suites live in `tests/13-srsim/`.

## Troubleshooting

- **Wrong template**: run `clab deploy --debug` and look for `Prepare SR OS config template`, which logs the template name, mode, node type, secure gRPC, and version.
- **Startup config rejected**: run `docker logs <node> 2>&1 | grep -E "MGMT_CORE|Configuration load failed"`. Errors such as obsolete enumeration values usually mean a version-dependent line was rendered for the wrong release.
- **Generated config**: see `<labdir>/<node>/<slot>/config/cf3/config.cfg`.
