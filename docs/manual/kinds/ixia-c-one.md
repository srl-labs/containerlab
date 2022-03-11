# Ixia-c-one

ixia-c-one is a re-packaged (as a single-container) flavor of multi-container application[ixia-c](https://github.com/open-traffic-generator/ixia-c).
It is identified with `ixia-c-one` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `ixia-c-one` node.


## Managing ixia-c-one nodes
Ixia-c-one node launched with containerlab can be managed via the following:

=== "bash"
    to connect to a `bash` shell of a running ixia-c-one container:
    ```bash
    docker exec -it <container-name/id> bash
    ```

## Interfaces mapping
ixia-c-one container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface

When containerlab launches ixia-c-one node, it will set IPv4/6 addresses as assigned by docker to the `eth0` interface and ixia-c-one node will boot with that addresses configured. Data interfaces `eth1+` need to be configured with IP addressing manually.

## Features and options
<TBD>

## Lab examples
The following labs feature ixia-c-one node:

- [Ixia-c-one and Arista cEOS](../../lab-examples/ixiacone-ceos.md)

## Known issues or limitations
<TBD>
