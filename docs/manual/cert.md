As more and more services move to "secure by default" behavior, it becomes important to simplify the PKI/TLS infrastructure provisioning in the lab environments. Containerlab embeds parts of [cfssl](https://github.com/cloudflare/cfssl) project to automate certificate generation and provisioning.

For [SR Linux](kinds/srl.md) nodes containerlab creates Certificate Authority (CA) and generates signed cert and key for each node of a lab. This makes SR Linux node to boot up with TLS profiles correctly configured and enable operation of a secured management protocol - gNMI.

!!!note
    For other nodes the automated TLS pipeline is not provided yet and can be addressed by contributors.

Apart from automated pipeline for certificate provisioning, containerlab exposes the following commands that can create a CA and node's cert/key:

* [`tools cert ca create`](../cmd/tools/cert/ca/create.md) - creates a Certificate Authority
* [`tools cert sign`](../cmd/tools/cert/sign.md) - creates certificate/key for a host and signs the certificate with CA

With these two commands users can easily create CA node certificates and secure the transport channel of various protocols. [This lab](https://clabs.netdevops.me/security/gnmitls/) demonstrates how with containerlab's help one can easily create certificates and configure Nokia SR OS to use it for secured gNMI communication.