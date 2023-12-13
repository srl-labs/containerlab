|                               |                                                                                 |
| ----------------------------- | ------------------------------------------------------------------------------- |
| **Description**               | A Cisco FTDv connected to two Alpine Linux Hosts                                |
| **Components**                | [Cisco FTDV][ftdv], [Multitool Alpine Linux][client]                            |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 4 <br/>:fontawesome-solid-memory: 8 GB            |
| **Topology file**             | [ftdv01.yml][topofile]                                                          |
| **Name**                      | ftdv01                                                                          |
| **Version information**[^2]   | `Cisco_Secure_Firewall_Threat_Defense_Virtual-7.2.5-208.qcow2`, `docker:24.0.6` |

## Description

This lab consists of one Cisco FTDv firewall connected to two Alpine Linux nodes.

```
client1<---->FTDv<---->client2
```

## Configuration

The FTDv node takes about 1-2 minutes to complete its start up. Check using "docker container ls" until the FTDv container shows up as "healthy".

```
# docker container ls
CONTAINER ID   IMAGE                                  COMMAND                  CREATED              STATUS                        PORTS                                       NAMES
5682d73984d1   vrnetlab/vr-ftdv:7.2.5                 "/launch.py --userna…"   34 minutes ago   Up 34 minutes (healthy)   22/tcp, 80/tcp, 443/tcp, 5000/tcp, 8305/tcp, 10000-10099/tcp   clab-ftdv01-ftdv1
1ebe3dae6846   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   34 minutes ago   Up 34 minutes             80/tcp, 443/tcp, 1180/tcp, 11443/tcp                           clab-ftdv01-client1
9726c9bb9e21   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   34 minutes ago   Up 34 minutes             80/tcp, 443/tcp, 1180/tcp, 11443/tcp                           clab-ftdv01-client2
```

### ftdv1

Log into the FTDv node using the Web UI and add the following configuration. Password is `Admin@123`.

1. Click "Skip device setup" on the initial screen.
2. In the dialog window "Are you sure you want to skip device setup?" check the "Start 90-day evaluation" box, select the "FTDv5 - Tiered" performance tier, and click "Confirm".
3. In the "Interfaces" menu configure GigabitEthernet0/0 with the `192.168.1.1/30` IP, and GigabitEthernet0/1 with the `192.168.2.1/30` IP.
4. Go to the "Policies" menu and add a test "allow all" policy (all fields should be left empty, and the action should be "allow").
5. Deploy pending changes.

### client1

The two clients should be configured with the correct IP addresses and a route to the other client via the FTDv node.
First attach to the container process `docker exec -it clab-ftdv01-client1 ash`

```
docker exec -it clab-ftdv01-client1 ash

# ip -br a show dev eth1
eth1@if3749      UP             192.168.1.2/30 fe80::a8c1:abff:feee:be5c/64

# ip r
default via 172.20.20.1 dev eth0
172.20.20.0/24 dev eth0 proto kernel scope link src 172.20.20.4
192.168.1.0/30 dev eth1 proto kernel scope link src 192.168.1.2
192.168.2.0/30 via 192.168.1.1 dev eth1
```

## Verification

Traceroute from client1 to client2 to verify the data-plane via the FTDv node.

### client1

```
# traceroute 192.168.2.2
traceroute to 192.168.2.2 (192.168.2.2), 30 hops max, 46 byte packets
 1  192.168.2.2 (192.168.2.2)  1.372 ms  0.909 ms  0.403 ms
```
  
[ftdv]: https://www.cisco.com/c/en/us/products/collateral/security/firepower-ngfw-virtual/threat-defense-virtual-ngfwv-ds.html
[client]: https://github.com/wbitt/Network-MultiTool
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/ftdv01/ftdv01.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
