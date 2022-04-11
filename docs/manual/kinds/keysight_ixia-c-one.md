# Keysight IXIA-C One

Keysight ixia-c-one is a single-container distribution of [ixia-c][ixia-c], which in turn is Keysight's reference implementation of [Open Traffic Generator API (OTG)][otg].

!!!info "What is IXIA-C?"
    Ixia-c is a modern, powerful and API-driven traffic generator designed to cater to the needs of hyperscalers, network hardware vendors and hobbyists alike.

    It is available **for free** and distributed / deployed as a multi-container application consisting of a [controller](https://hub.docker.com/r/ixiacom/ixia-c-controller), a [traffic-engine](https://hub.docker.com/r/ixiacom/ixia-c-traffic-engine) and an [app-usage-reporter](https://hub.docker.com/r/ixiacom/ixia-c-app-usage-reporter).

The corresponding node in containerlab is identified with `keysight_ixia-c-one` kind in the [topology file](../topo-def-file.md). Upon boot up, it comes up with:

- management interface `eth0` configured with IPv4/6 addresses as assigned by container runtime
- hostname assigned to the node name
- HTTPS service enabled on port 443 (for client SDK to push configuration and fetch metrics)

## Managing ixia-c-one nodes

ixia-c-one is a "docker in docker" container hosting two kinds of [ixia-c][ixia-c] containers internally:

- A set of containers acting as API endpoint and managing configuration across multiple test ports
- A set of containers bound to network interface (created by containerlab), treating it as a test port (i.e. for generating or processing traffic, emulating protocols, etc.)

Request and response to the API endpoint is driven by [Open Traffic Generator API][otg], and can be exercised in following two ways:

=== "Using SDK"
    Using SDK is the preferred way of interacting with OTG devices. Implementations listed in the [SDK](#sdk) chapter below provides references to SDK clients in different languages along with the examples.

    Test case designers create test cases using SDK in one of the supported languages and leverage native language toolchain to test/execute the tests. Being API-first, Open Traffic Generator compliant implementations provide full configuration flexibility over the API.

    SDK clients use HTTPS to interface with the OTG API.
=== "Using `curl`"
    ```bash
    # fetch configuration that was last pushed to ixia-c-one
    # clab-ixia-c-ixia-c-one is a container name allocated by clab for ixia node
    curl -kL https://clab-ixia-c-ixia-c-one/config

    # fetch flow metrics
    curl -kL https://clab-ixia-c-ixia-c-one/results/metrics -d '{"choice": "flow"}'
    ```

## SDK
Client SDK for configuring ixia-c is available in various languages, most prevalent being [gosnappi][gosnappi] and [snappi][snappi].

The example below demonstrates how [gosnappi][gosnappi] - Go-based SDK client - can be used to configure/run a traffic test along with evaluating results.

IPv4 Forwarding test is setup in the following way:

Endpoints: `OTG 1.1.1.1 -----> 1.1.1.2 DUT 2.2.2.1 ------> OTG 2.2.2.2`  
Static Route on DUT: `20.20.20.0/24 -> 2.2.2.2`  
TCP flow from OTG: `10.10.10.1 -> 20.20.20.1+`

1. [install](https://go.dev/doc/install) go and init a module
```bash
mkdir tests && cd tests
go mod init tests
```
1. Download gosnappi release. gosnappi version needs to be compatible to a given release of ixia-c and
can be checked at https://github.com/open-traffic-generator/ixia-c/releases
```bash
go get github.com/open-traffic-generator/snappi/gosnappi@v0.7.18
```
1. download a [basic IPv4 forwarding test](https://github.com/open-traffic-generator/snappi-tests/blob/b2b0d32e8d19589dc69ebd71eb5929d5f3c908f2/scripts/ipv4_forwarding.go)
```
curl -LO https://raw.githubusercontent.com/open-traffic-generator/snappi-tests/main/scripts/ipv4_forwarding.go
```
1. run the test with MAC address of the `1.1.1.2` interface which is located on the DUT as per the test diagram.
```
go run ipv4_forwarding.go -dstMac="<MAC address>"
```

## Interfaces mapping
ixia-c-one container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* The other interfaces (eth1, eth2, etc) are the data interfaces acting as traffic generation ports

When containerlab launches ixia-c-one node, it will set IPv4/6 addresses as assigned by docker to the `eth0` interface and ixia-c-one node will boot with that addresses configured.

Data interfaces `eth1+` need to be configured with IP addressing manually if needed (as in the Layer3 forwarding test example).
IP addresses are required when the test port needs to reply to ARP/ND queries from the Device Under Test. These addresses can be configured on non-`eth0` ports.

Examples below show how test designer can configure IP address on eth1 data port of a parent container named `ixiac-one`:

=== "IPv4 configuration"
    ```bash
    # set ipv4 addr
    docker exec -it ixiac-one bash -c "./ifcfg add eth2 2.2.2.2 24"

    # remove ipv4 addr
    docker exec -it ixiac-one bash -c "./ifcfg del eth2 2.2.2.2 24"
    ```
=== "IPv6 configuration"
    ```bash
    # set ipv6 addr on eth1 iface
    docker exec -it ixiac-one bash -c "./ifcfg add eth1 11::1 64"

    # remove ipv6 addr on eth1 iface
    docker exec -it ixiac-one bash -c "./ifcfg del eth1 11::1 64"
    ```

## Features and options
The free version of ixia-c supports generation of L2 and L3 traffic to test forwarding of Ethernet, IPv4 and IPv6 traffic by switches and routers. For technical support and queries , please log requests at [open-traffic-generator/ixia-c](https://github.com/open-traffic-generator/ixia-c/issues) or contact us via [Slack](https://ixia-c.slack.com/signup#/domain-signup).

The commercial version of ixia-c supports ARP/ND/Auto destination MAC resolution in data traffic, IPv4 and IPv6 BGP with IPv4 and IPv6 Routes and ISIS with IPv4 and IPv6 routes. Please contact Keysight support for further information regarding this if needed.

## Lab examples
The following labs feature Keysight ixia-c-one node:

- [Keysight ixia-c-one and Arista cEOS](../../lab-examples/ixiacone-ceos.md)

## Known issues or limitations
1. For L3 traffic tests using the free version , there is no in-built support of ARP and ND.  
   This can be worked around by manually setting IP address on the receive interface (as explained in Interfaces mapping section above) and by learning the MAC of the connected DUT using external means such as gnmi/ssh/reading it from CLI and using it when generating packets.  
   This limitation will be removed in the ixia-c-one free version in future releases where it is planned to support ARP/ND Request and Reply for emulated interfaces.  



[ixia-c]: https://github.com/open-traffic-generator/ixia-c  
[otg]: https://redocly.github.io/redoc/?url=https://raw.githubusercontent.com/open-traffic-generator/models/master/artifacts/openapi.yaml  
[gosnappi]: https://github.com/open-traffic-generator/snappi/tree/main/gosnappi  
[snappi]: https://pypi.org/project/snappi/
