---
search:
  boost: 4
---
# Check Point Cloudguard

Check Point Cloudguard virtualized security appliance is identified with `checkpoint_cloudguard` kind in the [topology file](../topo-def-file.md). It is built using [boxen](https://github.com/carlmontanari/boxen) project and essentially is a Qemu VM packaged in a docker container format.

## Getting Cloudguard image
Users can obtain the qcow2 disk image for Check Point Cloudguard VM from the [official download site](https://supportcenter.checkpoint.com/supportcenter/portal?eventSubmit_doGoviewsolutiondetails=&solutionid=sk158292). To build a containerlab-compatible container use [boxen](https://github.com/carlmontanari/boxen) project.

## Managing Check Point Cloudguard nodes

!!!note
    Containers with Check Point Cloudguard VM inside will take ~5min to fully boot.  
    You can monitor the progress with

    * `docker logs -f <container-name>` for boxen status reports
    *  and `docker exec -it <container-name> tail -f /console.log` to see the boot log messages.

Check Point Cloudguard node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running checkpoint_cloudguard container:
    ```bash
    docker exec -it <container-name/id> bash
    ```

    !!!note
        The shell access gives you access to the container that hosts the Qemu VM.

=== "CLI"
    to connect to the Cloudguard CLI
    ```bash
    ssh admin@<container-name/id/IP-addr>
    ```
=== "HTTPS"
    Cloudguard OS comes with HTTPS server running on boot. You can access the Web UI using https schema
    ```bash
    curl https://<container-name/id/IP-addr>
    ```

    You can expose container's 443 port with [`ports`](../nodes.md#ports) setting in containerlab and get access to the Web UI using your containerlab host IP.

!!!info
    Default login credentials: `admin:admin`

## Interfaces mapping
Check Point Cloudgard starts up with 8 available interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to the first data port of the VM
* `eth2+` - second and subsequent data interface

When containerlab launches Cloudguard node, it assigns a static `10.0.0.5` IPv4 address to the VM's `eth0` interface. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the Cloudguard using containerlab's assigned IP.

Data interfaces `eth1+` need to be configured with IP addressing manually using CLI or other available management interfaces.
