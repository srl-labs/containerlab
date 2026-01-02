---
search:
  boost: 4
kind_code_name: f5_bigip_ve
kind_display_name: F5 BIG-IP VE
---
# -{{ kind_display_name }}-

-{{ kind_display_name }}- is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). Aliases: `vr-f5_bigip_ve`, `vr-f5_bigip`. It is built using the [srl-labs/vrnetlab](../vrnetlab.md) project and runs a QEMU/KVM VM packaged in a docker container.

## Getting F5 BIG-IP VE disk image

BIG-IP VE is a proprietary image. You must download the BIG-IP VE KVM/qcow2 image from F5 (requires a registered account and appropriate entitlement) and build a vrnetlab container image yourself.

Refer to the build steps in the `f5_bigip` image documentation in the upstream vrnetlab project.

/// admonition
    type: warning
Containerlab does not ship BIG-IP VE images or licenses and does not provide guidance for bypassing licensing requirements.
///

## Managing F5 BIG-IP VE nodes

/// note
Containers with F5 BIG-IP VE inside will take a while to fully boot (often 10+ minutes).

You can monitor the progress with `docker logs -f <container-name>` and wait for the `Startup complete` log message.
///

Management networking uses vrnetlab management *passthrough*: the BIG-IP management IP is the same as the containerlab management IP for the node.

### Day-0 automation

This image relies on day-0 automation on first boot to make the node reachable without manual intervention:

- Management passthrough: configure BIG-IP management IP and default route to match the containerlab-assigned management IP/gateway.
- Credentials: handle the forced password-change prompt and set the CLI `root` and GUI `admin` password.

BIG-IP images commonly present an interactive forced password-change prompt early in the boot process; without this automation you would need to complete the dialog manually via the serial console (telnet) before the system is usable.

When a forced password-change prompt is encountered, vrnetlab sets the `admin` password (used for both CLI and GUI access) to `F5_NEW_PASSWORD` (see VRNetlab) (defaults to `Labl@b!234`) and saves the configuration.

Default credentials:

- BIG-IP GUI admin: `admin:Labl@b!234`
- BIG-IP CLI root: `root:Labl@b!234`

```

/// details | Example day-0 automation logs (successful startup)
    type: tip
These `docker logs` lines show a successful boot where the automation detects the login prompt, waits for `mcpd`, applies the containerlab management IP and default route, sets the `admin` (GUI) password, and saves the configuration:

```text
2026-01-02 03:20:22,628: launch         INFO Login prompt detected; marking VM as running
2026-01-02 03:20:22,628: launch         INFO Applying mgmt IP via console (handling forced password change if present)
2026-01-02 03:20:43,875: launch         WARNING Console provisioning timed out waiting for prompts
2026-01-02 03:21:13,917: launch         INFO console apply: echo READY
2026-01-02 03:21:13,959: launch         INFO Waiting for mcpd to be running (try 1/30)
2026-01-02 03:21:19,595: launch         INFO Waiting for mcpd to be running (try 2/30)
2026-01-02 03:21:24,859: launch         INFO Waiting for mcpd to be running (try 3/30)
2026-01-02 03:21:30,118: launch         INFO Waiting for mcpd to be running (try 4/30)
2026-01-02 03:21:35,368: launch         INFO Waiting for mcpd to be running (try 5/30)
2026-01-02 03:21:40,656: launch         INFO Waiting for mcpd to be running (try 6/30)
2026-01-02 03:21:45,946: launch         INFO Waiting for mcpd to be running (try 7/30)
2026-01-02 03:21:51,398: launch         INFO Waiting for mcpd to be running (try 8/30)
2026-01-02 03:21:56,666: launch         INFO Waiting for mcpd to be running (try 9/30)
2026-01-02 03:22:05,207: launch         INFO Waiting for mcpd to be running (try 10/30)
2026-01-02 03:22:05,754: launch         INFO mcpd is running
2026-01-02 03:22:05,754: launch         INFO console apply: tmsh -c 'modify sys global-settings mgmt-dhcp disabled'
2026-01-02 03:22:06,808: launch         INFO console apply: tmsh -c 'create sys management-ip 172.20.20.3/24'
2026-01-02 03:22:07,373: launch         INFO console apply: tmsh -c 'delete sys management-route default'
2026-01-02 03:22:07,656: launch         INFO console apply: tmsh -c 'modify sys management-route default gateway 172.20.20.1'
2026-01-02 03:22:07,906: launch         INFO console apply: tmsh -c 'create sys management-route default gateway 172.20.20.1'
2026-01-02 03:22:08,272: launch         INFO console apply: tmsh -c 'modify auth user admin password TestL@b!234'
2026-01-02 03:22:09,938: launch         INFO console apply: tmsh -c 'save sys config'
2026-01-02 03:22:20,128: launch         INFO Console mgmt provisioning complete
2026-01-02 03:22:20,128: launch         INFO Startup complete in: 0:03:04.868472
```
///

F5 BIG-IP VE nodes launched with containerlab can be managed via the following interfaces:

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
/// tab | Telnet (console)
The serial console is exposed on TCP port `5000`. This is useful when boot is failing or when you need console access.

```bash
docker exec -it <container-name/id> telnet 127.0.0.1 5000
# example:
# docker exec -it clab-test-clab-f5-f5_bigip-ve telnet 127.0.0.1 5000
```

///
/// tab | HTTPS
BIG-IP exposes the Web UI/API on port `443`:

```bash
https://<container-name/id/IP-addr>
```

To access the GUI using a host port, publish `443` with the [`ports`](../nodes.md#ports) setting (the `f5bigipve01` lab example uses `8443:443`):

```yaml
topology:
  nodes:
    bigip1:
      kind: f5_bigip_ve
      ports:
        - 8443:443
```

Then open:

```bash
https://localhost:8443
```

### Credentials overrides

You can override BIG-IP credentials with the node [`env`](../topo-def-file.md#environment-variables) map.

The vrnetlab image supports:

- `PASSWORD` (admin password; default in containerlab is `Labl@b!234`)

Example:

```yaml
topology:
  nodes:
    bigip1:
      kind: f5_bigip_ve
      image: vrnetlab/f5_bigip-ve:<version>
      env:
        PASSWORD: "MyStrongPassword"
```

## Interface naming

You can use [interface names](../topo-def-file.md#interface-naming) in the topology file like they appear in F5 BIG-IP VE.

The traffic interface naming convention is: `1.X`, where `X` is the port number.

With that naming convention in mind:

* `1.1` - first traffic (dataplane) interface
* `1.2` - second traffic (dataplane) interface, and so on...

The example ports above would be mapped to the following Linux interfaces inside the container running the F5 BIG-IP VE VM:

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

F5 BIG-IP VE is VM-based and requires hardware virtualization support on the host (KVM).

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

F5 BIG-IP VE requires a valid license to fully function.

Demo/evaluation licenses can be obtained via the official F5 licensing portal with a registered F5 account (subject to eligibility/terms).

/// admonition
    type: warning
This documentation does not provide guidance for bypassing licensing, and containerlab does not ship licenses.
///

## Troubleshooting

- If the node never reaches `Startup complete`, check `docker logs <container-name>` and confirm `/dev/kvm` is available and you have sufficient CPU/RAM.
- If SSH is up but the Web UI is not, wait a bit longer; HTTPS services may start after the initial login prompt is detected.
- If you override `PASSWORD` but it does not apply, ensure the value meets BIG-IP password complexity requirements; otherwise day-0 automation may reject it.
