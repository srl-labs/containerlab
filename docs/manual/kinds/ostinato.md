---
search:
  boost: 4
---
# Ostinato

[Ostinato](https://ostinato.org/) network traffic generator is currently identified with `linux` kind in the [topology file](../topo-def-file.md). This will change to its own kind in the future.

## Getting Ostinato image

Ostinato for containerlab image is a paid (_but inexpensive_) offering and can be obtained from the [Ostinato for Containerlab](https://ostinato.org/pricing/clab) section of the Ostinato website. Follow the instructions on the page to install the image.

## Topology definition

Add a topology definition for the Ostinato node in `.clab.yml` as shown below -

```yaml
topology:
  nodes:
    ost:
      kind: linux
      image: ostinato/ostinato:{tag}
      ports:
        - 5900:5900/tcp
        - 7878:7878/tcp
```

Replace `{tag}` above with the tag shown in the output of `docker images`

## Managing Ostinato nodes

Ostinato has a GUI and a Python API. The Ostinato image includes both the Ostinato agent (called _Drone_) which does the actual traffic generation and the Ostinato GUI that is used to configure and monitor the Drone agent.

The GUI is the primary way to work with Ostinato and is accessible over VNC. To use the Ostinato API, you will need [Ostinato PyApi](https://ostinato.org/pricing/pyapi).

/// tab | Using GUI
Once the lab is deployed, connect any VNC client to `<host-ip>:5900` - this will bring up the Ostinato GUI.

![Ostinato GUI](https://ostinato.org/images/ostinato-clab.png)

The GUI only lists the data interfaces and hence, `eth0` will not be included
///

/// tab | Using Python API
To manage the Ostinato node using the Ostinato Python API, you will typically write a script using the API and run it on the same or different host connecting to the Drone agent via the management interface
///

/// tab | shell
You can login to the shell of the Ostinato node for any troubleshooting, if required.
Ostinato **does not have any CLI or commands to generate traffic - use the GUI** (or API).

```
docker exec -it <container-name/id> bash
```
///

## Interfaces mapping

Ostinato container includes the following interfaces -

* `eth0` - management interface (should NOT be used for data traffic)
* `eth1+` - data interfaces to connect to other nodes for traffic generation

## File mounts

Ostinato allows you to save and load files - traffic streams, pcaps and session files. To ensure persistence of these files (after a lab is destroyed), mount a directory on the host to a directory inside the container as shown in the example below -

```yaml
topology:
  nodes:
    ost:
      kind: linux
      image: ostinato/ostinato:{tag}
      ports:
        - 5900:5900/tcp
        - 7878:7878/tcp
      binds:
        - /some/dir/on/host:/some/path/in/container
```

You can read more about [node binds](../nodes.md#binds)

## Lab examples

A simple [Ostinato and Nokia SR Linux](../../lab-examples/ost-srl.md) lab demonstrates a simple test topology, IPv4 traffic streams to verify L3 forwarding and a short video clip that shows it in action.

## More

You can find more information including how to run the Ostinato GUI natively on your Windows/MacOS laptop to manage the Ostinato agent running inside containterlab on the [Ostinato for Containerlab](https://ostinato.org/pricing/clab) page.
