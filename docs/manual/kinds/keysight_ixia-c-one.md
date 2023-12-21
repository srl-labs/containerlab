---
search:
  boost: 4
---
# Keysight Ixia-c-one

Keysight [Ixia-c-one][ixia-c-one] is a single-container distribution of [Ixia-c][ixia-c], a software traffic generator and protocol emulator with [Open Traffic Generator (OTG) API][otg].

/// note | What is Ixia-c?
Ixia-c is an agile and composable network test system designed for continuous integration. It is provides a modern, powerful and API-driven traffic generator designed to cater to the needs of network operators, vendors and hobbyists alike.

Ixia-c Community Edition is available **for free** with limitations. [Commercially licensed editions][ixia-c-licensing] are also available.
///

Users can pull Ixia-c-one container image from [Github Container Registry][ixia-c-one-image].

The corresponding node in containerlab is identified with `keysight_ixia-c-one` kind in the [topology file](../topo-def-file.md). Upon boot up, it comes up with:

- management interface `eth0` configured with IPv4/6 addresses as assigned by the container runtime
- hostname assigned to the node name
- HTTPS service enabled on port TCP/8443 (for client SDK to push configuration and fetch metrics)

## Managing Ixia-c-one nodes

Ixia-c-one provides an API endpoint that manages configuration across multiple test ports. Requests and responses to the API endpoint are defined by the [Open Traffic Generator API][otg] and can be exercised in the following two ways:

/// tab | Using SDK
Using SDK is the preferred way of interacting with OTG devices. Implementations listed in the [SDK](#sdk) chapter below provide references to SDK clients in different languages along with examples.

Test case designers create test cases using SDK in one of the supported languages and leverage native language toolchain to test/execute the tests. Being API-first, Open Traffic Generator compliant implementations provide full configuration flexibility over the API.

SDK clients use HTTPS to interface with the OTG API.
///

/// tab | Using `curl`

```bash
# fetch configuration that was last pushed to ixia-c-one
# assuming 'clab-ixiac01-ixia-c' is a container name allocated by containerlab for the Ixia-c node
curl -kL https://clab-ixiac01-ixia-c:8443/config

# fetch flow metrics
curl -kL https://clab-ixiac01-ixia-c:8443/monitor/metrics -d '{"choice": "flow"}'
```

///

## SDK

Client SDK for Open Traffic Generator API is available in various languages, most prevalent being [gosnappi][gosnappi] for Go and [snappi][snappi] for Python.

## Lab examples

The following labs feature Keysight ixia-c-one node:

- [Keysight Ixia-c and Nokia SR Linux](../../lab-examples/ixiacone-srl.md)

[ixia-c]: https://ixia-c.dev/
[ixia-c-one]: https://ixia-c.dev/deployments-containerlab/
[ixia-c-one-image]: https://github.com/orgs/open-traffic-generator/packages/container/package/ixia-c-one
[otg]: https://otg.dev
[gosnappi]: https://github.com/open-traffic-generator/snappi/tree/main/gosnappi
[snappi]: https://pypi.org/project/snappi/
[ixia-c-licensing]: https://ixia-c.dev/licensing/
