---
kind_code_name: spirent_stc
kind_display_name: Spirent TestCenter
---
# -{{ kind_display_name }}-

-{{ kind_display_name }}- is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).

-{{ kind_display_name }}- (STC) is a containerised version of the Spirent traffic generator. It supports up to 9 test interfaces.

## Managing -{{ kind_display_name }}- nodes

/// tab | Using STC Client
Once the STC is node is up in `healthy` state. You can connect using the STC client.

See the [Client access](#client-access) section for more info.
///

/// tab | Container shell
For troubleshooting you can access the container shell. You cannot generate any traffic using this interface.

Use the following command:

```sh
docker exec -it <container name> sh
```

///

## Interfaces

The STC container uses the following interface mapping:

|     Interface     |         Usage        |
| ----------------- | -------------------- |
| `admin0`          | Management interface |
| `port1` - `port9` | Test port interfaces |

A maximum of 9 test ports are supported.

Interfaces are defined in the topology file as `port1-9` (inclusive), this matches the port naming you shall see in the Test Center client when reserving ports.

## Client access

As the test center client runs on Windows, it is required to establish connectivity between the client and `-{{ kind_code_name }}-` node.

### Port forwarding

The suggested method is to use port forwarding, which exposes the required ports from the `-{{ kind_code_name }}-` node on your containerlab host system.

#### Sample topology

Below is a sample topology with the port publishing on the node: `my-stc`.

```yaml
name: port-publish
topology:
  nodes:
    my-stc:
      kind: spirent_stc
      image: stc:5.50.2906
      ports:
        - 80:80
        - 40004:40004/udp
        - 40005:40005/udp
        - 51204:51204/udp
```

### Routing

Another method is to establish L3 connectivity either via some sort of tunneling or installing a static route to enable the client system to reach the lab [management network](../network.md#management-network).

While containerlab attempts to allow [external access](../network.md#management-network) to the management network, ensure that no firewall rules/filtering could be preventing external access.

#### Windows example

The static route is typically installed for the whole containerlab management subnet with a next-hop of the containerlab host IP address.

The command prompt/`cmd` command to add the static route on Windows is:

```cmd
route add <management subnet> MASK <dotted decimal subnet mask> <containerlab host IP>
```

For example, if my management subnet was `172.20.20.0/24` and my containerlab host IP is `192.168.1.200`. The command would be:

```cmd
route add 172.20.20.0 MASK 255.255.255.0 192.168.1.200
```

## Host optimisations

/// note
While the below configurations on the host system may increase -{{ kind_display_name }}-, most NOSes supported by Containerlab are not designed for high throughput packet forwarding and likely implement dataplane performance limiting.
///

Some host optimisations are suggested to increase performance of a -{{ kind_display_name }}-.

- All power saving (SpeedStep, C1 states, etc.) and turbo boost options should be disabled on the host system.


### Recommended sysctls

The following sysctls are suggested to be set on the host to improve Tx/Rx rates.

```bash
sudo sysctl -w net.core.rmem_max=67108864
sudo sysctl -w net.core.wmem_max=67108864
```

To make these persistent across reboots, add these to your host sysctl configuration via `/etc/sysctl.conf` or as a drop in `.conf` file under `/etc/sysctl.d/`:

```
net.core.rmem_max = 67108864
net.core.wmem_max = 67108864
```
