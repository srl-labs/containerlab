---
search:
  boost: 4
kind_code_name: rtbrick_bngblaster
kind_display_name: RtBrick BNG Blaster
---
# -{{ kind_display_name }}-

[-{{ kind_display_name }}-](https://github.com/rtbrick/bngblaster) is an open-source network tester for access and routing protocols. It emulates subscribers (PPPoE, IPoE, L2TP, IPTV) and routing protocols (IS-IS, OSPF, LDP, BGP including EVPN), and verifies millions of traffic streams. -{{ kind_display_name }}- node is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md); `bngblaster` is accepted as a short alias.

The container image runs the [BNG Blaster controller](https://github.com/rtbrick/bngblaster-controller), which manages the test instances and provides a REST API and web UI. Version 1.1.1 or newer of the controller is required, which is part of the image since BNG Blaster 1.1.0. The image `ghcr.io/rtbrick/bngblaster:1.2.0` or newer is recommended.

## Getting -{{ kind_display_name }}- image

The image is publicly available in the GitHub Container Registry as [`ghcr.io/rtbrick/bngblaster`](https://github.com/rtbrick/bngblaster/pkgs/container/bngblaster). It is used by default if the node has no `image` configured.

```bash
docker pull ghcr.io/rtbrick/bngblaster:latest
```

## Managing -{{ kind_display_name }}- nodes

/// tab | Web UI and REST API
The controller serves a web UI and REST API on port 8001, from the lab host for example at `http://clab-<lab-name>-<node-name>:8001`. To reach it from a remote browser, publish the port with a different host port per node:

```yaml
    bngblaster:
      kind: rtbrick_bngblaster
      ports:
        - 8001:8001
```

///

/// tab | CLI
`bngblaster-cli` sends commands to the control socket of the running test:

```bash
docker exec -it clab-<lab-name>-<node-name> \
  bngblaster-cli /var/bngblaster/clab/run.sock session-counters
```

///

/// tab | Shell
```bash
docker exec -it clab-<lab-name>-<node-name> bash
```

///

## Interface naming

-{{ kind_display_name }}- uses the interface names of the links as they are, which are referenced in the BNG Blaster configuration:

* `eth0` - management interface connected to the containerlab management network
* `eth1+` - data interfaces, any other name can be used as well

## Features and options

### Startup configuration

With the [`startup-config`](../nodes.md#startup-config) property, a user sets the path to a BNG Blaster configuration file (JSON). It is copied to `/var/bngblaster/clab/config.json` in the container, and the controller starts this test instance (`clab`) automatically once all links of the node are attached.

```yaml
topology:
  nodes:
    bngblaster:
      kind: rtbrick_bngblaster
      startup-config: pppoe.json
```

Without a startup-config, or with [`suppress-startup-config`](../nodes.md#suppress-startup-config), only the controller is started, and tests are created and started through the web UI or REST API.

As for other kinds, an existing `config.json` from a previous deployment is kept, so changes to the startup-config file take effect only after `containerlab destroy --cleanup`, with `containerlab deploy --reconfigure`, or with [`enforce-startup-config`](../nodes.md#enforce-startup-config) set for the node.

The start arguments of the test, for example to write a report or log file, are read from `/var/bngblaster/clab/run.json` if present:

```yaml
      binds:
        - run.json:/var/bngblaster/clab/run.json:ro
```

```json
{
    "report": true,
    "report_flags": ["sessions", "streams"],
    "logging": true,
    "logging_flags": ["info", "error"]
}
```

### Results

The node's lab directory (`clab-<lab-name>/<node-name>`) is mounted as `/var/bngblaster`, the instance directory of the controller. The configuration, logs and the report of the test (`clab/run_report.json`) are kept there after the lab is destroyed, unless the lab directory is removed with `containerlab destroy --cleanup` or `containerlab deploy --reconfigure`.

### Stopping the test

The controller stops the test when the container is stopped, and BNG Blaster writes the report in all of the following cases:

* `containerlab destroy`, with or without `--graceful`, and the removal of the node by `containerlab apply`: the node is stopped before any link or node of the lab is removed, also those of the device under test, so that BNG Blaster tears down its sessions gracefully (PPPoE: LCP Terminate-Request and PADT; routing sessions are closed). A teardown that takes longer than 30 seconds is ended at once, still with the report. For large tests, where the device under test should see every session closed, raise `BNGBLASTERCTRL_STOP_TIMEOUT` (e.g. `5m`) and the containerlab [`--timeout`](../../cmd/deploy.md#timeout) (default 2 minutes) to more than that plus 10 seconds, e.g. `containerlab destroy --timeout 6m`.

* `containerlab stop`: the links are removed before the node is stopped, so the sessions can not be torn down gracefully. The controller detects the missing interfaces and stops the test at once.
* The test ends by itself or is stopped through the web UI, REST API or `bngblaster-cli`.

`containerlab start` and `containerlab restart` start the test again once all links are attached. This removes the results of the previous run, so copy them before if needed.

### Health check

The node has a default health check, which is healthy while the test is running or after it finished without error, and unhealthy while waiting for the links, while the test starts, or if it failed. Other nodes can therefore wait for the test with [`wait-for`](../nodes.md#stages) and the `healthy` stage. The check is replaced by a [`healthcheck`](../nodes.md#healthcheck) configured for the node.

### Environment variables

The kind sets the following controller variables, which can be overridden with the node's `env`:

| Variable                         | Description                                          | Default                            |
| -------------------------------- | ---------------------------------------------------- | ---------------------------------- |
| `BNGBLASTERCTRL_AUTOSTART`       | Instance started automatically                        | `clab` if a startup-config is set  |
| `BNGBLASTERCTRL_WAIT_INTERFACES` | Number of interfaces to wait for before the start     | number of links of the node        |

All other controller options are available as `BNGBLASTERCTRL_<OPTION>` variables as well, for example `BNGBLASTERCTRL_STOP_TIMEOUT` (default `30s`) for the graceful teardown when the node is stopped, see the [controller documentation](https://github.com/rtbrick/bngblaster-controller).

### IPv6

The kind sets the sysctl `net.ipv6.conf.default.disable_ipv6=1`, so that the kernel does not use IPv6 on the data interfaces, which BNG Blaster handles itself. The management interface keeps IPv6. The sysctl is not set with `network-mode: host` or `network-mode: container:<name>`, where the node has no network namespace of its own.

## Lab examples

* [BNG Blaster PPPoE](../../lab-examples/bngblaster01.md) - PPPoE server and clients emulated back to back by one BNG Blaster node.
* [BNG Blaster and FRR IS-IS](../../lab-examples/bngblaster02.md) - emulated IS-IS topology advertised to FRR, verified with traffic streams through FRR.

## Known issues and limitations

* containerlab links have an MTU of 9500 by default, while BNG Blaster network interfaces use an MTU of 1500 by default and IS-IS accepts PDUs of up to 1497 bytes. Routers size IS-IS PDUs to the link MTU, for example padded hellos, and depending on the implementation LSPs and CSNPs, which are then rejected, so that the adjacency does not come up or the database is incomplete. Either configure `jumbo-frames`, the `mtu` of the interface and the IS-IS `max-pdu-size` of the BNG Blaster according to the link MTU, as in the [IS-IS lab example](../../lab-examples/bngblaster02.md) (requires BNG Blaster 1.2.0 or newer), or set the [`mtu`](../topo-def-file.md#links) of links to routing peers to 1500.
* Without `"interfaces": {"jumbo-frames": true}` in the BNG Blaster configuration, frames larger than about 4 KB are not received completely.

## Contact

The kind is maintained by RtBrick, contact <bngblaster@rtbrick.com>.
