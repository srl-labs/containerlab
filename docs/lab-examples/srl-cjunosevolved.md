|                               |                                                                                          |
| ----------------------------- | ---------------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Juniper cJunosEvolved                       |
| **Components**                | [Nokia SR Linux][srl], [Juniper cJunosEvolved][cjunosevolved]                            |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 6 <br/>:fontawesome-solid-memory: 12 GB                    |
| **Topology file**             | [srlcjunosevo.clab.yml][topofile]                                                        |
| **Name**                      | srlcjunosevo                                                                             |
| **Version information**[^2]   | `containerlab:0.68.0`, `srlinux:23.7.1`, `cjunosevolved:25.2R1.3-EVO`, `docker-ce:27.1.1,` |

## Description

A lab consists of an SR Linux node connected with Juniper vJunosEvolved via three point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `clab` docker network.

## Use cases

The nodes are provisioned with a basic interface configuration for three interfaces they are connected with. Pings between the nodes should work out of the box using all three interfaces.

## SSH to the management port

The management port IP is assigned by containerlab. The `FXP0ADDR` token in `lab-examples/srlcjunosevo/cjunosevo.cfg' file is replaced by the management IP at cJunosEvolved startup and provides ssh connectivity to it.

```
# docker inspect clab-srlcjunosevo-cevo  | grep "IPAddress"
            "SecondaryIPAddresses": null,
            "IPAddress": "",
                    "IPAddress": "172.20.20.3",

# The admin password is `admin@123`
# ssh admin@172.20.20.3
(admin@172.20.20.3) Password:
Last login: Wed Jun 11 19:16:01 2025 from 172.20.20.1
--- JUNOS 25.2R1.3-EVO Linux (none) 5.15.164-10.22.33.18-yocto-standard-juniper-16986-g445edc512bb4 #1 SMP PREEMPT Tue May 13 12:28:49 UTC 2025 x86_64 x86_64 x86_64 GNU/Linux
admin@re0>
```


[srl]: ../manual/kinds/srl.md
[cjunosevolved]: ../manual/kinds/cjunosevolved.md
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/srlcjunosevo/srlcjunosevo.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using the above versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
