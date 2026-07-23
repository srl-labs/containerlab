---
search:
  boost: 4
kind_code_name: light_olt
kind_display_name: Light OLT
tags:
  - Kind
  - Container kind
---
# -{{ kind_display_name }}-

The [-{{ kind_display_name }}-](https://github.com/abelperezr/olt-light) emulator is identified by the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). The kind supplies Containerlab defaults for running the emulator and connecting its Linux data interfaces.

-{{ kind_display_name }}- emulates a Nokia Lightspan FX OLT with separate iHUB, Shelf, and LT management planes. It also provides a Lightspan-style eCLI and emulation services for ONU DHCP, optical diagnostics, and IPFIX telemetry.

## Getting -{{ kind_display_name }}- image

Containerlab does not distribute the -{{ kind_display_name }}- image. Pull the version used by the topology from GHCR:

```bash
docker pull ghcr.io/abelperezr/olt-light:0.0.2
```

The integration has been validated with version `0.0.2`. Reference that version explicitly in the topology:

```yaml
topology:
  nodes:
    olt:
      kind: -{{ kind_code_name }}-
      image: ghcr.io/abelperezr/olt-light:0.0.2
```

/// warning | Startup time
A Light OLT node normally takes between two and three minutes to initialize all
management planes. Wait until the container reports a `healthy` status before
connecting through eCLI or NETCONF.
///

## Architecture

Each management plane runs a separate Netopeer2 instance, Sysrepo repository, and YANG context inside the same container:

| Plane | Function | NETCONF port |
| ----- | -------- | :----------: |
| iHUB  | Network-facing services and uplink configuration | 831 |
| Shelf | Shelf hardware and LT inventory | 832 |
| LT1   | First line card | 833 |
| LT2–LT4 | Optional line cards selected at startup | 834–836 |

The eCLI and emulation services use these datastores as their source of truth. Configuration written through eCLI or NETCONF is therefore visible to the other services running in the node.

## Managing -{{ kind_display_name }}- nodes

-{{ kind_display_name }}- nodes launched with Containerlab can be managed through eCLI, NETCONF, or the Linux shell.

/// tab | eCLI
The emulated CLI is available over SSH on port 22:

```bash
ssh admin@<node-name>
```
///

/// tab | NETCONF
The emulator exposes one NETCONF endpoint for each management plane. The ports are listed in the [architecture table](#architecture). Connect to iHUB, Shelf, and LT1 with:

```bash
ssh admin@<node-name> -p 831 -s netconf
ssh admin@<node-name> -p 832 -s netconf
ssh admin@<node-name> -p 833 -s netconf
```
///

/// tab | Linux shell
Use the container runtime to open a shell:

```bash
docker exec -it <container-name> bash
```
///

### Credentials

The default credentials are:

* username: `admin`
* password: `admin`

## Interface naming

The management network uses `eth0`. Topologies can use the Lightspan port
names directly. The kind translates each name to the Linux interface used
inside the container:

| Topology endpoint | Linux interface |
| ----------------- | --------------- |
| `1/2/1`           | `eth1`          |
| `1/1/1`           | `eth2`          |
| `1/1/2`           | `eth3`          |
| `1/1/3`           | `eth4`          |
| `1/1/4`           | `eth5`          |

For example:

```yaml
links:
  - endpoints: ["olt:1/1/1", "sros:1/1/c2/1"]
```

## Features and options

### LT layout

LT1 is always the `FGLT-D` plane included in the image and listens on NETCONF
port 833. Its card model cannot be changed at runtime.

Use `OLT_LT_SLOTS` to enable LT2 through LT4 and select either `FGLT-D` or
`FWLT-C` for those slots. Slots 2 through 4 correspond to NETCONF ports 834
through 836. Include `1=FGLT-D` in the layout so the Shelf inventory matches
the fixed LT1 plane.

```yaml
env:
  OLT_LT_SLOTS: "1=FGLT-D,2=FWLT-C,3=FWLT-C,4=FWLT-C"
```

If the variable is omitted, the emulator starts only the fixed LT1 plane.

`OLT_LT_PLANES` is a simpler alternative that accepts a value from `1` through
`4`. It creates that number of LT planes using `FGLT-D`. When `OLT_LT_SLOTS`
is present, the highest configured slot determines the required number of
planes.

### Environment variables

The following image variables are useful in Containerlab topologies:

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `OLT_LT_SLOTS` | Keeps LT1 as `FGLT-D` and selects `FGLT-D` or `FWLT-C` for LT2–LT4. | `1=FGLT-D` |
| `OLT_LT_PLANES` | Selects the number of `FGLT-D` LT planes when a per-slot layout is not required. | `1` |
| `ONU_DHCP_POLL` | Sets the ONU DHCP reconciliation interval in seconds. | `20` |
| `ONU_OPTICS_ENABLED` | Set to `0` to disable optical diagnostics emulation. | `1` |
| `IPFIX_EMU_ENABLED` | Set to `0` to disable the synthetic IPFIX exporter. | `1` |

### ONU inventories

The autofind inventory for LT1 is read from `/seeds/onts_oper.xml`. Optional LT clones use `/seeds/onts_oper_lt2.xml` through `/seeds/onts_oper_lt4.xml`; a clone uses the LT1 file when its plane-specific file is absent.

Bind-mount the inventories when they need to be edited without rebuilding the image:

```yaml
binds:
  - ./seeds/onts_oper.xml:/seeds/onts_oper.xml:ro
  - ./seeds/onts_oper_lt2.xml:/seeds/onts_oper_lt2.xml:ro
```

The emulator reloads an inventory after the host file changes.

Each inventory is an `ietf-interfaces` operational-state document. Define one
`interface` entry for every configured GPON or XGS-PON channel termination and
place its detected ONUs below
`onus-present-on-local-channel-termination`. A single LT inventory can contain
both GPON and XGS-PON interfaces:

```xml
<interfaces-state xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
  <interface>
    <name>CT_LT2_PON1_1_GPON</name>
    <type xmlns:bbf-xponift="urn:bbf:yang:bbf-xpon-if-type">bbf-xponift:channel-termination</type>
    <oper-status>up</oper-status>
    <channel-termination xmlns="urn:bbf:yang:bbf-xpon">
      <onus-present-on-local-channel-termination xmlns="urn:bbf:yang:bbf-xpon-onu-state">
        <onu>
          <detected-serial-number>ALCL00000001</detected-serial-number>
          <onu-presence-state xmlns:bbf-xpon-onu-types="urn:bbf:yang:bbf-xpon-onu-types">bbf-xpon-onu-types:onu-present-and-no-v-ani-known-and-in-o5</onu-presence-state>
          <onu-detected-datetime>2026-07-08T20:27:06Z</onu-detected-datetime>
        </onu>
      </onus-present-on-local-channel-termination>
    </channel-termination>
  </interface>
  <interface>
    <name>CT_LT2_PON1_1_XGS</name>
    <type xmlns:bbf-xponift="urn:bbf:yang:bbf-xpon-if-type">bbf-xponift:channel-termination</type>
    <oper-status>up</oper-status>
    <channel-termination xmlns="urn:bbf:yang:bbf-xpon">
      <onus-present-on-local-channel-termination xmlns="urn:bbf:yang:bbf-xpon-onu-state">
        <onu>
          <detected-serial-number>ALCL00000041</detected-serial-number>
          <onu-presence-state xmlns:bbf-xpon-onu-types="urn:bbf:yang:bbf-xpon-onu-types">bbf-xpon-onu-types:onu-present-and-no-v-ani-known-and-in-o5</onu-presence-state>
          <onu-detected-datetime>2026-07-08T20:27:06Z</onu-detected-datetime>
        </onu>
      </onus-present-on-local-channel-termination>
    </channel-termination>
  </interface>
</interfaces-state>
```

The interface name must exactly match the channel termination configured in
the target LT. Use a unique serial number for every ONU, repeat the `onu`
element as needed, and use an RFC 3339 value for `onu-detected-datetime`.

### ONU subscriber emulation

The ONU DHCP service discovers enabled VLAN sub-interfaces in every LT datastore. A subscriber is created only when the VSI VLAN matches an enabled iHUB v-VPLS with an enabled SAP on a mapped physical uplink. The service then creates the VLAN interface and subscriber macvlan before exchanging DHCPv4 or DHCPv6 packets with the BNG.

Follow subscriber events with:

```bash
docker logs -f <container-name> 2>&1 | grep onu-dhcp
```

Successful sessions report `via=v-vpls` followed by a `DHCPv4 ACK` or `DHCPv6 REPLY` message.

### Health check

The kind installs a health check that verifies:

* SSH on port 22;
* iHUB NETCONF on port 831;
* Shelf NETCONF on port 832;
* every LT NETCONF endpoint selected by `OLT_LT_SLOTS` or `OLT_LT_PLANES`.

The default timing is a 30-second start period, a 5-second interval, a 3-second timeout, and 24 retries. A node-level [`healthcheck`](../nodes.md#healthcheck) replaces these defaults.

### Restart and link changes

The default restart policy is `no`. This prevents a runtime restart from losing data interfaces that Containerlab already injected into the network namespace.

The kind applies link changes in `live` mode. The emulator can detect data interfaces added after container creation without recreating the node.

### Startup and saved configuration

The kind supports Containerlab's [`startup-config`](../nodes.md#startup-config) and [`save`](../../cmd/save.md) operations. It accepts two startup formats:

* `.txt` is a sectioned eCLI overlay intended for readable startup configuration.
* `.tgz` is an exact multi-plane backup generated by `clab save`.

A text overlay provides a readable and maintainable initial configuration:

```text
[SHELF]
hardware component Board-LT1
class board
parent Slot-LT1
parent-rel-pos 1

[IHUB]
service vpls 10
admin-state enable
customer 1
v-vpls true
vlan 10

[LT1]
interfaces interface LT1_PON1
type channel-group
enabled true
```

Reference the file directly from the topology:

```yaml
startup-config: ./configs/olt.txt
```

The supported section names are `[SHELF]`, `[IHUB]`, and `[LT1]` through `[LT4]`. Empty lines, lines beginning with `#`, and lines containing only `!` are ignored. The loader enters the correct configuration mode and performs one commit at the end of each section.

Before applying a text overlay, the image snapshots every running management plane. If parsing, an eCLI command, or a commit fails, all planes are restored to that snapshot and the OLT continues starting with its image or persistent defaults. A failure to restore the snapshot is fatal because a consistent configuration can no longer be guaranteed.

The saved `.tgz` bundle contains a manifest and one complete XML configuration for iHUB, Shelf, and every enabled LT. ONU autofind inventories are operational data and are not included.

Use a saved bundle when an exact restoration is required:

```yaml
topology:
  nodes:
    olt:
      kind: light_olt
      image: ghcr.io/abelperezr/olt-light:0.0.2
      startup-config: ./clab-bng-olt/olt/config/light-olt-startup.tgz
      env:
        OLT_LT_SLOTS: "1=FGLT-D,2=FWLT-C"
```

The LT count and card layout in the bundle must exactly match the topology. The image validates the manifest, restores each `running` datastore before starting the management planes, and copies the result to the `startup` datastore.

Save the running configuration of every plane with:

```bash
clab save -t <topology-file>
```

The bundle is written to Containerlab's default lab directory at `<lab-directory>/<node-name>/config/light-olt-startup.tgz`. For example, the `bng-olt` lab stores the OLT bundle at `./clab-bng-olt/olt/config/light-olt-startup.tgz`.

If both a generated bundle in the lab directory and `startup-config` exist, Containerlab preserves the generated bundle. Deploy with `--reconfigure` to replace it with the explicitly referenced file.

### Nokia Altiplano integration

Light OLT supports integration with Nokia Altiplano through the companion
[NETCONF proxy image](https://github.com/abelperezr/olt-light/pkgs/container/olt-proxy).
Altiplano must connect to the proxy, not directly to the OLT NETCONF ports. The
proxy presents the capabilities and YANG library expected by the device
extensions. The proxy is a separate node and is not started or configured
automatically by the `-{{ kind_code_name }}-` kind.

#### Supported profile

The supported profile is aligned with these 25.12 device extensions:

* `device-extension-ls-fx-fant-g-fx4-25.12-660`
* `device-extension-ls-fx-fglt-d-25.12-660`
* `device-extension-ls-fx-fwlt-c-25.12-660`
* `device-extension-ls-fx-ihub-fant-g-fx4-25.12-660`

The corresponding blueprint is:

```text
downloaded-ls-fx-25.12-25.12.2-REL_281
```

This list defines the reference combination for the integration. Other
blueprint and device-extension versions or combinations are not declared
compatible by this documentation.

#### Obtaining the artifacts

Nokia Altiplano, the blueprint, and the device extensions listed above are not
included in this repository or in the Light OLT images. Nokia licenses,
installation packages, and official documentation are not included either.

Obtain the relevant information, software, blueprint, and device extensions
directly from Nokia through the appropriate authorized channels. Their names
are shown here only to identify the emulator's compatibility profile.

## Lab examples

The [Light OLT example](../../lab-examples/light-olt.md) connects OLT port
`1/1/1` to port `1/1/c2/1` of a Nokia SR-SIM BNG.

## Further reading

The [eCLI guide](https://abelperezr.github.io/olt-light/en/docs/how-to/cli-guide)
provides configuration examples for the Lightspan-style eCLI.

## Limitations

* The emulator supports up to four LT planes, on NETCONF ports 833 through 836.
* The kind manages only the OLT container; external systems such as a BNG, Nokia Altiplano, and the companion NETCONF proxy must be defined separately.
