# Light OLT with Nokia SR-SIM

This example deploys a four-LT Light OLT, a Nokia SR-SIM BNG, and the optional
NETCONF proxy used by Nokia Altiplano. The BNG port `1/1/c2/1` connects to the
OLT endpoint `1/1/1`, which the `light_olt` kind maps to `eth2` inside the
container.

## Prerequisites

Install Containerlab and Docker, then pull the Light OLT images:

```bash
docker pull ghcr.io/abelperezr/olt-light:0.1.0
docker pull ghcr.io/abelperezr/olt-proxy:0.0.1
```

Obtain the SR-SIM image and a valid license directly from Nokia. Make the image
available locally as `nokia_srsim:25.10.R2` and save the license as:

```text
configs/license/SR_SIM_license.txt
```

Allow approximately 1–1.5 GB of RAM for the four-LT OLT in addition to the
resources required by SR-SIM.

## Files

The example uses the following layout:

```text
light-olt-srsim/
├── light-olt-srsim.clab.yml
├── configs/
│   ├── license/
│   │   └── SR_SIM_license.txt
│   ├── olt/
│   │   └── olt.txt
│   └── sros/
│       └── bng.txt
├── persist/
│   └── olt-proxy/
│       └── data/
└── seeds/
    ├── onts_oper.xml
    └── onts_oper_gpon_xgs.xml
```

`configs/olt/olt.txt` is a sectioned eCLI startup configuration. It does not
need explicit `config`, `configure global`, or `commit` commands. The loader
enters the correct context and commits each section.

The ONU inventory files populate the LT autofind operational data. They can be
copied from the Light OLT repository and adjusted for the lab.

### Create GPON and XGS-PON inventories

Create one operational-state XML file per LT. The same file can contain both
GPON and XGS-PON channel terminations:

```xml
<interfaces-state xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
  <interface>
    <name>CT_LT2_PON1_1_GPON</name>
    <type xmlns:bbf-xponift="urn:bbf:yang:bbf-xpon-if-type">bbf-xponift:channel-termination</type>
    <oper-status>up</oper-status>
    <channel-termination xmlns="urn:bbf:yang:bbf-xpon">
      <onus-present-on-local-channel-termination xmlns="urn:bbf:yang:bbf-xpon-onu-state">
        <onu>
          <detected-serial-number>ALCL00000001</detected-serial-number>
          <onu-presence-state xmlns:bbf-xpon-onu-types="urn:bbf:yang:bbf-xpon-onu-types">bbf-xpon-onu-types:onu-present-and-no-v-ani-known-and-in-o5</onu-presence-state>
          <onu-detected-datetime>2026-07-08T20:27:06Z</onu-detected-datetime>
        </onu>
      </onus-present-on-local-channel-termination>
    </channel-termination>
  </interface>
  <interface>
    <name>CT_LT2_PON1_1_XGS</name>
    <type xmlns:bbf-xponift="urn:bbf:yang:bbf-xpon-if-type">bbf-xponift:channel-termination</type>
    <oper-status>up</oper-status>
    <channel-termination xmlns="urn:bbf:yang:bbf-xpon">
      <onus-present-on-local-channel-termination xmlns="urn:bbf:yang:bbf-xpon-onu-state">
        <onu>
          <detected-serial-number>ALCL00000041</detected-serial-number>
          <onu-presence-state xmlns:bbf-xpon-onu-types="urn:bbf:yang:bbf-xpon-onu-types">bbf-xpon-onu-types:onu-present-and-no-v-ani-known-and-in-o5</onu-presence-state>
          <onu-detected-datetime>2026-07-08T20:27:06Z</onu-detected-datetime>
        </onu>
      </onus-present-on-local-channel-termination>
    </channel-termination>
  </interface>
</interfaces-state>
```

For each detected ONU:

1. use a serial number that is unique across the LT;
2. repeat the `onu` element below the applicable GPON or XGS-PON interface;
3. set `onu-detected-datetime` to an RFC 3339 timestamp;
4. ensure the interface name exactly matches the channel termination configured
   through eCLI or NETCONF.

LT1 reads `/seeds/onts_oper.xml`. LT2 through LT4 read
`/seeds/onts_oper_lt2.xml`, `/seeds/onts_oper_lt3.xml`, and
`/seeds/onts_oper_lt4.xml`, respectively. The topology maps the combined
`onts_oper_gpon_xgs.xml` example to LT2.

## Topology

```yaml
name: bng-olt
prefix: ""

mgmt:
  network: bng-olt
  ipv4-subnet: 172.30.30.0/24

topology:
  kinds:
    nokia_srsim:
      image: nokia_srsim:25.10.R2
      license: configs/license/SR_SIM_license.txt

  nodes:
    sros:
      kind: nokia_srsim
      mgmt-ipv4: 172.30.30.2
      type: sr-7
      components:
        - slot: A
        - slot: B
        - slot: 1
          type: iom5-e
          env:
            NOKIA_SROS_MDA_1: me6-100gb-qsfp28
            NOKIA_SROS_SFM: m-sfm6-7/12
        - slot: 2
          type: iom4-e-b
          env:
            NOKIA_SROS_MDA_1: isa2-bb
            NOKIA_SROS_SFM: m-sfm6-7/12
      startup-config: configs/sros/bng.txt
      ports:
        - 56612:22

    olt:
      kind: light_olt
      image: ghcr.io/abelperezr/olt-light:0.1.0
      mgmt-ipv4: 172.30.30.10
      binds:
        - ./seeds/onts_oper.xml:/seeds/onts_oper.xml:ro
        - ./seeds/onts_oper_gpon_xgs.xml:/seeds/onts_oper_lt2.xml:ro
      env:
        OLT_LT_SLOTS: "1=FGLT-D,2=FWLT-C,3=FWLT-C,4=FWLT-C"
      ports:
        - 56613:22
      startup-config: configs/olt/olt.txt

    # Required only when Nokia Altiplano manages the OLT.
    olt-proxy:
      kind: linux
      image: ghcr.io/abelperezr/olt-proxy:0.0.1
      mgmt-ipv4: 172.30.30.11
      binds:
        - ./persist/olt-proxy/data:/data
      env:
        UPSTREAM_HOST: 172.30.30.10

  links:
    - endpoints: ["olt:1/1/1", "sros:1/1/c2/1"]
```

The topology file is available in the
[`light-olt-srsim` lab directory](https://github.com/srl-labs/containerlab/tree/main/lab-examples/light-olt-srsim).

The `olt-proxy` node is not part of the subscriber data path. Remove it when
Altiplano integration is not required.

## OLT startup configuration

At minimum, the iHUB section must define a valid customer and v-VPLS. The SAP
must use the same OLT port and VLAN as the subscriber service:

```text
[IHUB]
service vpls 10
admin-state enable
customer 1
v-vpls true
vlan 10
sap 1/1/1:10 admin-state enable

[LT1]
# Add the PON, ONU, VSI, QoS, and profile configuration here.
```

If any section fails, Light OLT restores every management plane to its
pre-overlay state and continues with the image defaults. Review the configuration
messages before troubleshooting the BNG:

```bash
docker logs olt 2>&1 | grep '^\[config\]'
```

A successful load includes:

```text
[config] applying eCLI startup overlay: /clab/config/light-olt-startup.txt
[config] eCLI startup overlay applied successfully
```

## Deploy the lab

From the example directory, run:

```bash
clab dep -t light-olt-srsim.clab.yml
```

The OLT can take approximately two minutes to initialize all management planes.
Check the lab:

```bash
clab ins -t light-olt-srsim.clab.yml
```

The OLT is ready when its state is `running` and its health is `healthy`.

Connect to the OLT eCLI:

```bash
ssh admin@172.30.30.10
```

Connect to SR-SIM:

```bash
ssh admin@172.30.30.2
```

## Verify subscriber traffic

Follow the subscriber daemon:

```bash
docker logs -f olt 2>&1 | grep onu-dhcp
```

The daemon processes a subscriber only when:

1. the LT contains an enabled VLAN sub-interface;
2. its VLAN matches an enabled iHUB v-VPLS;
3. the v-VPLS contains an enabled SAP on `1/1/1`;
4. the BNG configuration accepts the subscriber session.

A successful session reports `via=v-vpls` followed by a `DHCPv4 ACK` or
`DHCPv6 REPLY`.

## Destroy the lab

```bash
clab des -t light-olt-srsim.clab.yml
```

See the [`light_olt` kind reference](../manual/kinds/light_olt.md) for interface
mapping, NETCONF ports, startup formats, and Altiplano integration.
