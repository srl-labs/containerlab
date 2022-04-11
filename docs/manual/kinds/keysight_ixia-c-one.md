# Keysight IXIA-C One

Keysight ixia-c-one is a single-container distribution of [ixia-c][ixia-c], which in turn is Keysight's reference implementation of [Open Traffic Generator API][otg].

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
    The example below uses Go-based [gosnappi][gosnappi] SDK client.

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
    3. download a basic IPv4 forwarding test
    ```
    curl -LO https://raw.githubusercontent.com/open-traffic-generator/snappi-tests/main/scripts/ipv4_forwarding.go
    ```
    1. run the test with MAC address obtained in previous step
    ```
    go run ipv4_forwarding.go -dstMac="<MAC address>"
    ```
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

## Interfaces mapping
ixia-c-one container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* The other interfaces are the data interfaces which are created using same name as provided in the containerlab topology yaml file. 

When containerlab launches ixia-c-one node, it will set IPv4/6 addresses as assigned by docker to the `eth0` interface and ixia-c-one node will boot with that addresses configured.

Data interfaces `eth1+` need to be configured with IP addressing manually if needed (as in the Layer3 forwarding test example).
This is needed when the test port needs to reply to ARP/ND queries from the Device Under Test.

- To configure an IPv4 address on any data link e.g. eth2 (Could be eth1 or eth3 as well, but never on eth0):
    ```bash
    docker exec -it clab-ixia-c-ixia-c-one bash -c "./ifcfg add eth2 2.2.2.2 24"
    ```
- To unset an IPv4 address on any data link e.g. eth2
    ```bash
    docker exec -it clab-ixia-c-ixia-c-one bash -c "./ifcfg del eth2 2.2.2.2 24"
    ```
- To configure an IPv6 address on any data link e.g. eth1 :
    ```bash
    docker exec -it clab-ixia-c-ixia-c-one bash -c "./ifcfg add eth1 11::1 64"
    ```
- To unset an IPv4 address on any data link e.g. eth2
    ```bash
    docker exec -it clab-ixia-c-ixia-c-one bash -c "./ifcfg del eth1 11::1 64"
    ```

## Features and options
The free version of ixia-c supports generation of L2 and L3 traffic to test forwarding of Ethernet, IPv4 and IPv6 traffic by switches and routers. For technical support and queries , please log requests at https://github.com/open-traffic-generator/ixia-c/issues or contact us @ https://ixia-c.slack.com/signup#/domain-signup .

The commercial version of ixia-c supports ARP/ND/Auto destination MAC resolution in data traffic, IPv4 and IPv6 BGP with IPv4 and IPv6 Routes and ISIS with IPv4 and IPv6 routes. Please contact Keysight support for further information regarding this if needed.

## Lab examples
The following labs feature Keysight ixia-c-one node:

- [Keysight ixia-c-one and Arista cEOS](../../lab-examples/ixiacone-ceos.md)

## Known issues or limitations
1. For L3 traffic tests using the free version , there is no in-built support of ARP and ND.  
   This can be worked around by manually setting IP address on the receive interface (as explained in Interfaces mapping section above) and by learning the MAC of the connected DUT using external means such as gnmi/ssh/reading it from CLI and using it when generating packets.  
   This limitation will be removed in the ixia-c-one free version in future releases where it is planned to support ARP/ND Request and Reply for emulated interfaces.  

2. Every time a containerlab topology with an ixia-c-one node is removed, it leaves behind a persistent storage.  
   If there are no other persistent unlinked storages on your system, you can remove it by removing all unlinked persistent storages by giving the command:
    ```bash
    docker volume prune
    ```

   If you wish to be very safe:
    ```bash
    # get the volume corresponding ot ixia-c-one node
    docker inspect clab-ixia-c-ixia-c-one
    # note the volume name from output
    "Mounts": [
                {
                    "Type": "volume",
                    "Name": "d1e87f85d3352bfb9dac3f8bac8eebee738503802cb9380966b5c4805bd791da",
                    "Source": "/var/lib/docker/volumes/d1e87f85d3352bfb9dac3f8bac8eebee738503802cb9380966b5c4805bd791da/_data",
                    "Destination": "/var/lib/docker", 

    # remove the volume
    docker volume remove d1e87f85d3352bfb9dac3f8bac8eebee738503802cb9380966b5c4805bd791da
    ```

    > This can be optionally be fixed in containerlab with one of the two approaches below:  
    > i) During docker run pass the --rm flag when starting the containers, or  
    > ii) During docker rm pass the -v flag when removing the containers.


[ixia-c]: https://github.com/open-traffic-generator/ixia-c  
[otg]: https://redocly.github.io/redoc/?url=https://raw.githubusercontent.com/open-traffic-generator/models/master/artifacts/openapi.yaml  
[gosnappi]: https://github.com/open-traffic-generator/snappi/tree/main/gosnappi  
[snappi]: https://pypi.org/project/snappi/
