---
search:
  boost: 4
kind_code_name: f5_bigip_ve
kind_display_name: F5 BIG-IP VE
---
# -{{ kind_display_name }}-

-{{ kind_display_name }}- is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is built using the [srl-labs/vrnetlab](../vrnetlab.md) project and runs a QEMU/KVM VM packaged in a docker container.

## Getting -{{ kind_display_name }}- disk image

BIG-IP VE is a proprietary image. You must download the BIG-IP VE KVM/qcow2 image from F5 (requires a registered account and appropriate entitlement) and build a vrnetlab container image yourself.

Refer to the build steps in the `f5_bigip` image documentation in the upstream vrnetlab project.

/// admonition
    type: warning
Containerlab does not ship BIG-IP VE images or licenses and does not provide guidance for bypassing licensing requirements.
///

## Managing -{{ kind_display_name }}- nodes

/// note
Containers with -{{ kind_display_name }}- inside will take a while to fully boot (often 10+ minutes).

You can monitor the progress with `docker logs -f <container-name>` and wait for the `Startup complete` log message.
///

Management networking uses vrnetlab management *passthrough*: the BIG-IP management IP is the same as the containerlab management IP for the node.

-{{ kind_display_name }}- nodes launched with containerlab can be managed via the following interfaces:

/// tab | bash
to connect to a `bash` shell of a running BIG-IP VE container:

```bash
docker exec -it <container-name/id> bash
```

///
/// tab | CLI via SSH
to connect to the BIG-IP CLI via SSH:

```bash
ssh admin@<container-name/id/IP-addr>
```

///
/// tab | HTTPS
BIG-IP exposes the Web UI/API on port `443`:

```bash
https://<container-name/id/IP-addr>
```

///
/// note
Default credentials:

- BIG-IP admin: `admin:admin`
- BIG-IP root: `root:default`
///

### Credentials overrides

You can override BIG-IP credentials with the node [`env`](../topo-def-file.md#environment-variables) map.

The vrnetlab image supports:

- `USERNAME` (defaults to `admin`; current day-0 automation targets the built-in `admin` user)
- `PASSWORD` (admin password; defaults to `admin`)
- `ROOT_PASSWORD` (root password; defaults to `default`)
- `F5_NEW_PASSWORD` (used only if a forced password-change prompt appears on first boot)

Containerlab sets `F5_NEW_PASSWORD` to `PASSWORD` by default to keep the effective credentials deterministic even if a forced password change is triggered. Note that in a forced password-change flow, both `admin` and `root` end up using `F5_NEW_PASSWORD`.

Example:

```yaml
topology:
  nodes:
    bigip1:
      kind: f5_bigip_ve
      image: vrnetlab/f5_bigip-ve:<version>
      env:
        PASSWORD: "MyStrongPassword"
        F5_NEW_PASSWORD: "MyStrongPassword"
        ROOT_PASSWORD: "MyRootPassword"
```

## Interface naming

You can use [interface names](../topo-def-file.md#interface-naming) in the topology file like they appear in -{{ kind_display_name }}-.

The traffic interface naming convention is: `1.X`, where `X` is the port number.

With that naming convention in mind:

* `1.1` - first traffic (dataplane) interface
* `1.2` - second traffic (dataplane) interface, and so on...

The example ports above would be mapped to the following Linux interfaces inside the container running the -{{ kind_display_name }}- VM:

* `eth0` - management interface connected to the containerlab management network (passthrough)
* `eth1` - first dataplane interface, mapped to the first traffic interface of the VM (rendered as `1.1`)
* `eth2+` - second and subsequent dataplane interfaces, mapped to the second and subsequent traffic interfaces of the VM (rendered as `1.2` and so on)

Data interfaces `1.1+` need to be configured with IP addressing manually using the CLI/GUI.

You can also use `ethX` interface names directly. Both examples below map to the same dataplane interface:

```yaml
links:
  - endpoints: ["bigip1:1.1", "dut:eth1"]
  - endpoints: ["bigip1:eth1", "dut:eth2"]
```

## Features and options

### Resource sizing

-{{ kind_display_name }}- is VM-based and requires hardware virtualization support on the host (KVM).

Default sizing knobs (set via node `env`):

- `QEMU_SMP` (vCPU count; default `4`)
- `QEMU_MEMORY` (RAM in MB; default `8192`)
- `QEMU_CPU` (CPU model; default `host`)

### Connection mode

You can set `CONNECTION_MODE` (default `tc`). The vrnetlab launch script supports: `tc|bridge|ovs-bridge|macvtap`.

/// note
`macvtap` requires access to `/dev`. Containerlab mounts `/dev:/dev` automatically when `CONNECTION_MODE=macvtap` is set.
///

## Licensing

-{{ kind_display_name }}- requires a valid license to fully function.

Demo/evaluation licenses can be obtained via the official F5 licensing portal with a registered F5 account (subject to eligibility/terms).

/// admonition
    type: warning
This documentation does not provide guidance for bypassing licensing, and containerlab does not ship licenses.
///

## Troubleshooting

- If the node never reaches `Startup complete`, check `docker logs <container-name>` and confirm `/dev/kvm` is available and you have sufficient CPU/RAM.
- If SSH is up but the Web UI is not, wait a bit longer; HTTPS services may start after the initial login prompt is detected.
- If you changed `PASSWORD` and hit a forced password-change prompt, explicitly set `F5_NEW_PASSWORD` and redeploy.
