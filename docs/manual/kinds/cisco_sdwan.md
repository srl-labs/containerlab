---
search:
  boost: 4
kind_code_name: cisco_sdwan
kind_display_name: Cisco SD-WAN Controllers
---
# Cisco SD-WAN

Cisco SD-WAN controller components are identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is built using [srl-labs/vrnetlab](https://github.com/srl-labs/vrnetlab/tree/master/cisco/sdwan-components) project and essentially is a Qemu VM packaged in a container format.

This kind supports all three Cisco SD-WAN controller components:

- **vManage**: Orchestration and management
- **vSmart**: Control plane controller
- **vBond**: Orchestrator/validator

## Hardware resource requirements

| Component | vCPU | RAM   | Disk       |
|-----------|------|-------|------------|
| vManage   | 1    | 16 GB | 30 GB + 50 GB data |
| vSmart    | 1    | 4 GB  | 30 GB      |
| vBond     | 1    | 2 GB  | 30 GB      |

## Managing -{{ kind_code_name }}- nodes

/// note
SD-WAN components boot in 5-10 minutes. Monitor progress with:

```bash
docker logs -f <container-name>
```

Wait for `System Ready` or `All daemons up` message.
///

/// tab | SSH

`ssh admin@<node-name>`
Password: `admin`
///
/// tab | bash
To connect to a `bash` shell:

```bash
docker exec -it <container-name/id> bash
```

///

### Credentials

Default credentials: `admin:admin`

## Component Types

Specify the component type using the `type` parameter:

- `manager` - vManage orchestrator
- `controller` - vSmart control plane
- `validator` - vBond orchestrator

```yaml
topology:
  nodes:
    sdwan-manager:
      kind: cisco_sdwan
      type: manager
      image: vrnetlab/cisco_sdwan-manager:20.16.1

    sdwan-controller:
      kind: cisco_sdwan
      type: controller
      image: vrnetlab/cisco_sdwan-controller:20.16.1

    sdwan-validator:
      kind: cisco_sdwan
      type: validator
      image: vrnetlab/cisco_sdwan-validator:20.16.1
```

### Using the Validator as vEdge

The validator(vBond) image can be repurposed as a vEdge device. 
To do this you set `type: validator` and set `cpu: 2`.
The default validator configuration does permit reconfiguring the node as a vEdge, but you can supply a regular vEdge bootstrap configuration via `startup-config` (both zcloud.xml and bootstrap configs are supported).

```yaml
topology:
  nodes:
    vedge1:
      kind: cisco_sdwan
      type: validator
      image: vrnetlab/cisco_sdwan-validator:20.16.1
      cpu: 2
      memory: 2Gb
      startup-config: vedge1-bootstrap.cfg
```

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in the device.

The interface naming convention is: `ethX`, where `X` starts at 1.

- `eth0` - Management interface (VPN 512)
- `eth1` - First transport interface (VPN 0)
- `eth2+` - Additional transport interfaces

```yaml
links:
  - endpoints: ["sdwan-manager:eth1", "sdwan-validator:eth1"]
```

## Features and options

### Startup Configuration

The cisco_sdwan kind supports two configuration file formats:

#### zCloud XML Configuration

```yaml
topology:
  nodes:
    sdwan-manager:
      kind: cisco_sdwan
      type: manager
      startup-config: sdwan-manager-config.xml
```

Example zCloud XML (`sdwan-manager-config.xml`):

```xml
<config xmlns="http://tail-f.com/ns/config/1.0">
  <system xmlns="http://viptela.com/system">
    <personality>vmanage</personality>
    <device-model>vmanage</device-model>
    <host-name>my-vmanage</host-name>
    <aaa>
      <user>
        <name>admin</name>
        <password>MyPassword</password>
        <group>netadmin</group>
      </user>
    </aaa>
  </system>
  <vpn xmlns="http://viptela.com/vpn">
    <vpn-instance>
      <vpn-id>512</vpn-id>
      <interface>
        <if-name>eth0</if-name>
        <ip>
          <dhcp-client>true</dhcp-client>
        </ip>
      </interface>
    </vpn-instance>
  </vpn>
</config>
```

#### Full Cloud-Init Configuration

```yaml
topology:
  nodes:
    sdwan-manager:
      kind: cisco_sdwan
      type: manager
      startup-config: sdwan-manager-cloud-init.yaml
```

Example cloud-init (`sdwan-manager-cloud-init.yaml`):

```yaml
#cloud-config
write_files:
- path: /etc/default/personality
  content: "vmanage\n"
- path: /etc/confd/init/zcloud.xml
  content: |
    <config xmlns="http://tail-f.com/ns/config/1.0">
      <!-- zCloud XML content here -->
    </config>
```

If no `startup-config` is provided, the component will boot with auto-generated default configuration.

## Lab examples

/// tab | Basic SD-WAN Controllers

```yaml
name: sdwan-controllers

topology:
  nodes:
    sdwan-manager:
      kind: cisco_sdwan
      type: manager
      image: vrnetlab/cisco_sdwan-manager:20.16.1

    sdwan-controller:
      kind: cisco_sdwan
      type: controller
      image: vrnetlab/cisco_sdwan-controller:20.16.1

    sdwan-validator:
      kind: cisco_sdwan
      type: validator
      image: vrnetlab/cisco_sdwan-validator:20.16.1

  links:
    - endpoints: ["sdwan-manager:eth1", "sdwan-validator:eth1"]
    - endpoints: ["sdwan-controller:eth1", "sdwan-validator:eth1"]
```

///

/// tab | SD-WAN with Edge Devices

```yaml
name: sdwan-fabric

topology:
  nodes:
    sdwan-manager:
      kind: cisco_sdwan
      type: manager
      image: vrnetlab/cisco_sdwan-manager:20.16.1

    sdwan-controller:
      kind: cisco_sdwan
      type: controller
      image: vrnetlab/cisco_sdwan-controller:20.16.1

    sdwan-validator:
      kind: cisco_sdwan
      type: validator
      image: vrnetlab/cisco_sdwan-validator:20.16.1

    edge1:
      kind: cisco_c8000v
      type: controller  # SD-WAN managed mode
      image: vrnetlab/vr-c8000v:17.11.01a

  links:
    - endpoints: ["sdwan-manager:eth1", "sdwan-validator:eth1"]
    - endpoints: ["sdwan-controller:eth1", "sdwan-validator:eth1"]
    - endpoints: ["edge1:Gi2", "sdwan-validator:eth1"]
```

///

/// tab | With Custom Configuration

```yaml
name: sdwan-custom

topology:
  nodes:
    sdwan-manager:
      kind: cisco_sdwan
      type: manager
      startup-config: configs/sdwan-manager-zcloud.xml
      image: vrnetlab/cisco_sdwan-manager:20.16.1

    sdwan-controller:
      kind: cisco_sdwan
      type: controller
      startup-config: configs/sdwan-controller-cloud-init.yaml
      image: vrnetlab/cisco_sdwan-controller:20.16.1

    sdwan-validator:
      kind: cisco_sdwan
      type: validator
      image: vrnetlab/cisco_sdwan-validator:20.16.1
```

///
