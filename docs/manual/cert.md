As more and more services move to "secure by default" behavior, it becomes important to simplify the PKI/TLS infrastructure provisioning in the lab environments.  
Containerlab tries to ease the process of certificate provisioning providing the following features:

1. Automated certificate provisioning for lab nodes.
2. Simplified CLI for CA and end-node keys generation.
3. Ability to use custom/external CA.

## Automated certificate provisioning

Automated certificate provisioning is a two-stage process. First, containerlab creates a Certificate Authority (CA) and generates a certificate and key for it, storing these artifacts in a [lab directory](conf-artifacts.md) in the `.tls` directory. Then, containerlab generates a certificate and key for each node of a lab and signs it with the CA. The signed certificate and key are then installed on the node.

!!!note
    Currently, automated installation of a node certificate is implemented only for [Nokia SR Linux](kinds/srl.md).

### CA certificate

When generating CA certificate and key, containerlab can take in the following optional parameters:

* `.settings.certificate-authority.key-size` - the size of the key in bytes, default is 2048
* `.settings.certificate-authority.validity-duration` - the duration of the certificate. For example: `10m`, `1000h`. Max unit is hour. Default is `8760h` (1 year)

### Node certificates

The decision to generate node certificates is driven by either of the following two parameters:

1. node kind
2. `issue` boolean parameter under `node-name.certificate` section.

For SR Linux nodes the `issue` parameter is set to `true` and can't be changed. For other node kinds the `issue` parameter is set to `false` by default and can be [overridden](nodes.md#certificate) by the user.

## Simplified CLI for CA and end-node keys generation

Apart automated pipeline for certificate provisioning, containerlab exposes the following commands that can create a CA and node's cert/key:

* [`tools cert ca create`](../cmd/tools/cert/ca/create.md) - creates a Certificate Authority
* [`tools cert sign`](../cmd/tools/cert/sign.md) - creates certificate/key for a host and signs the certificate with CA

With these two commands users can easily create CA node certificates and secure the transport channel of various protocols. [This lab](https://clabs.netdevops.me/security/gnmitls/) demonstrates how with containerlab's help one can easily create certificates and configure Nokia SR OS to use it for secured gNMI communication.

## External CA

Users who require more control over the certificate generation process can use an existing external CA. Containerlab needs to be provided with the CA certificate and key. The CA certificate and key must be provided via `.settings.certificate-authority.[key]|[cert]` configuration parameters.

```yaml
name: ext-ca
settings:
  certificate-authority:
    cert: /path/to/ca.crt
    key: /path/to/ca.key
```

When using an external CA, containerlab will not generate a CA certificate and key. Instead, it will use the provided CA certificate and key to sign the node certificates.

The paths can be provided in absolute or relative form. If the path is relative, it is relative to the directory where clab file is located.

In addition to setting External CA files via `settings` section, users can also set the following environment variables:

* `CLAB_CA_CERT_FILE` - path to the CA certificate
* `CLAB_CA_KEY_FILE` - path to the CA key
