---
search:
  boost: 4
kind_code_name: openwrt
kind_display_name: OpenWRT
---
# OpenWRT

[OpenWRT](https://openwrt.org/) is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Getting OpenWRT image

To build -{{ kind_display_name }}- docker container image follow the instructions from the [vrnetlab repo](https://github.com/srl-labs/vrnetlab/tree/master/openwrt).

## Example

```yaml
name: openwrt

topology:
  nodes:
    openwrt:
      kind: openwrt
      image: vrnetlab/openwrt_openwrt:24.10.0
      mgmt-ipv4: 172.20.20.12                             # optional
      mgmt_ipv6: 2001:172:20:20::12                       # optional
      ports:
        - 8080:80                                         # required for LuCI web interface (HTTP); adjust host ports if running multiple nodes or based on your setup
        - 8443:443                                        # required for LuCI web interface (HTTPS); adjust host ports if running multiple nodes or based on your setup
      env:
        USERNAME: root                                    # default: root
        PASSWORD: mypassword                              # default: VR-netlab9
        CLAB_MGMT_PASSTHROUGH: "false"                    # default: "false"
        PACKET_REPOSITORY_DNS_SERVER: 8.8.8.8             # default 8.8.8.8
        PACKET_REPOSITORY_DOMAINS: "example.com"          # additional repository domains (space-separated); creates a host route via the MGMT interface
        PACKAGES: "tinc htop tcpdump btop luci-proto-gre" # installed on boot if not already present
```
