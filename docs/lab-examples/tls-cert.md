|                               |                                                                                                     |
| ----------------------------- | --------------------------------------------------------------------------------------------------- |
| **Description**               | Securing gNMI with containerlab generated certificates                                              |
| **Components**                | Nokia SR OS                                                                                         |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 6 GB                                |
| **Topology file**             | [cert01.clab.yml][topofile]                                                                         |
| **Version information**[^2]   | `containerlab:0.12.0`, `vr-sros:21.2.R1`, `docker-ce:19.03.13`, `vrnetlab:0.2.3`[^3], `gnmic:0.9.0` |

## Description
Nowadays more and more protocols require a secured transport layer for their operation where TLS is king. Creating a Certificate Authority, public/private keys, certificate signing requests and signing those was a mundane task that most network engineers tried to avoid...

But thanks to the opensource projects like [cfssl](https://github.com/cloudflare/cfssl) it is now less painful to overcome the difficulties of bootstrapping the PKI infra at least in the lab setting. Containerlab embeds parts of cfssl to expose what we consider a critical set of commands that enable our users to quickly set up TLS enabled transports.

This lab demonstrates how containerlab' helper commands instantly create the necessary certificates for CA and the SR OS router to enable TLS-secured gNMI communication.

## Lab deployment
Before we start generating certificates, let's deploy this simple lab which consists of a single Nokia SR OS node with no data interfaces whatsoever.

```
clab dep -t ~/cert01.clab.yml
```

Write down the IP address that container engine assigned to our node, as we will use it in the certificate phase.

```
+---+----------------+--------------+--------------------------+---------+-------+---------+----------------+----------------------+
| # |      Name      | Container ID |          Image           |  Kind   | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+----------------+--------------+--------------------------+---------+-------+---------+----------------+----------------------+
| 1 | clab-cert01-sr | 183c82e1a033 | vrnetlab/vr-sros:21.2.R1 | vr-sros |       | running | 172.20.20.2/24 | 2001:172:20:20::2/80 |
+---+----------------+--------------+--------------------------+---------+-------+---------+----------------+----------------------+
```

## Certificate generation
As promised, containerlab aims to provide a necessary tooling for users to enable TLS transport. In short, we need to create a CA which will sign the certificate of the SR OS node that we will also create.
For that we will leverage the following containerlab commands:

* [`tools cert ca create`](../cmd/tools/cert/ca/create.md) - creates a Certificate Authority
* [`tools cert sign`](../cmd/tools/cert/sign.md) - creates certificate/key for a host and signs the certificate with CA

### Create CA
First we need to create a Certificate Authority that will be able to sign a node's certificate. Leveraging the default values that `ca create` command embeds, we can be as short as this:

```bash
# create CA certificate and key in the current working dir
containerlab tools cert ca create
```

As a result of this command we will have `ca.pem` and `ca-key.pem` files generate in our current working directory. That is all it takes to create a CA.

### Create and sign node certificate
Next is the node certificate that we need to create and sign with the CA created before. Again, this is pretty simple, we just need to specify the DNS names and IP addresses we want this certificate to be valid for.

Since containerlab creates persistent DNS names for the fully qualified node names, we know that DNS name of our router is `clab-cert01-sr`, which follow the pattern of `clab-<lab-name>-<node-name>`.

We will also make our certificate to be valid for the IP address of the node. To get the IP address of the node refer to the summary table which containerlab provides when the lab deployment finishes. In our case the IP was `172.20.20.2`.

Knowing the DNS and IP of the node we can create the certificate and key and immediately sign it with the Certificate Authority created earlier. All in one command!

```bash
containerlab tools cert sign --ca-cert ca.pem --ca-key ca-key.pem \
             --hosts clab-cert01-sr,172.20.20.2

INFO[0000] Creating and signing certificate:
  Hosts=["clab-cert01-sr" "172.20.20.2"], CN=containerlab.srlinux.dev,
  C=Internet, L=Server, O=Containerlab, OU=Containerlab Tools 
```

Here we leveraged [`tools cert sign`](../cmd/tools/cert/sign.md) command that firstly inits the CA by using the its files `ca.pem` and `ca-key.pem` and then creates a node certificate for the DNS and IP names provided via `hosts` flag. 

Now, in our working directory we have the signed node's certificate with the file names `cert.pem`, `cert-key.pem` and CA cert and key from the previous step.

Two short commands and you are good to go and configure SR OS to use them.

## Configuring SR OS
### Transferring certificate and key

At a minimum we need to transfer the node certificate and key. An extra mile would be to also transfer the CA files to the node, but we will not do that in this lab.

We will transfer the certificate files with SCP, but you can choose any other means:

```
scp cert-key.pem admin@clab-cert01-sr:cf3:/
scp cert.pem admin@clab-cert01-sr:cf3:/
```

### Importing certificate and key
SR OS needs the certificates to be imported after they are copied to the flash card. For that we need to switch to use the Classic CLI notation with `//` command prefix:

```
//admin certificate import type cert input cf3:/cert.pem output cert.pem format pem
//admin certificate import type key input cf3:/cert-key.pem output cert-key.pem format pem
```

When certificates are imported, they are copied to a system `system-pki` directory on the flash card:

```
[/]
A:admin@sr# file list system-pki

Volume in drive cf3 on slot A is SROS VM.

Volume in drive cf3 on slot A is formatted as FAT32

Directory of cf3:\system-pki

03/26/2021  08:50p      <DIR>          ./
03/26/2021  08:50p      <DIR>          ../
03/26/2021  08:51p                1256 cert-key.pem
03/26/2021  08:50p                1095 cert.pem
               2 File(s)                   2351 bytes.
               2 Dir(s)               683569152 bytes free.
```

This command verifies that our two files - node' certificate and a matching private key - have been imported successfully.

### Certificate profile
Next step is to create a certificate profile that will bring the imported certificate file and a its private key under a single logical construct.

```
/configure system security tls cert-profile sr-cert-prof  entry 1 certificate-file cert.pem
/configure system security tls cert-profile sr-cert-prof entry 1 key-file cert-key.pem
/configure system security tls cert-profile sr-cert-prof admin-state enable
```

### Ciphers list
Proceed with creating a ciphers list that SR OS will use when negotiating TLS with. We choose a single cipher, though many are available on SR OS to match your client capabilities.

```
/configure system security tls server-cipher-list "ciphers" cipher 1 name tls-rsa-with3des-ede-cbc-sha
```

### Server TLS profile
Finishing step is configuring the specific SR OS construct called "server-tls-profile". It sets which TLS profile, ciphers (and optionally CRL) to use for a specific TLS server configuration.

```
/configure system security tls server-tls-profile sr-server-tls-prof cert-profile "sr-cert-prof" admin-state enable
/configure system security tls server-tls-profile sr-server-tls-prof
```

### Configuring secured gRPC
Now when TLS objects are all created, we can make gRPC services on SR OS make use of the TLS. To do that, we override the default unsecured gRPC that `vr-sros` uses with a one that uses the tls-server-profile we created earlier:

```
/configure system grpc tls-server-profile "sr-server-tls-prof"
commit
```

=== "gRPC config before"
    ```
    (pr)[/configure system grpc]
    A:admin@sr# info
        admin-state enable
        allow-unsecure-connection
        gnmi {
            auto-config-save true
        }
        rib-api {
            admin-state enable
        }
    ```
=== "gRPC config after"
    ```
    *(pr)[/configure system grpc]
    A:admin@sr# info
        admin-state enable
        tls-server-profile "sr-server-tls-prof"
        gnmi {
            auto-config-save true
        }
        rib-api {
            admin-state enable
        }
    ```

Now gRPC services will require TLS to be used by the clients, let's verify it.

## Verification
We will use gnmic CLI to issue gNMI RPCs to check if TLS is now really enforced and used.

First, let's use the DNS name that our SR OS node an entry in /etc/hosts for[^4].

```
gnmic -a clab-cert01-sr -u admin -p admin --tls-ca ca.pem capabilities
gNMI version: 0.7.0
supported models:
  - nokia-conf, Nokia, 21.2.R1
  - nokia-state, Nokia, 21.2.R1
  - nokia-li-state, Nokia, 21.2.R1
  - nokia-li-conf, Nokia, 21.2.R1
supported encodings:
  - JSON
  - BYTES
  - PROTO
```

Note here, that we use the [`--tls-ca`](https://gnmic.kmrd.dev/global_flags/#tls-ca) flag of gnmic to make sure that we verify the server's (router's) certificate by checking it with a CA certificate.

If you remember, when we [created the router' certificate](#create-and-sign-node-certificate) we specified not only its DNS name, but also the IP address. This allows us to use management IP address with gNMI and still being able to verify the router's certificate:

```
gnmic -a 172.20.20.2 -u admin -p admin --tls-ca ca.pem capabilities
gNMI version: 0.7.0
<SNIP>
```

Feel free to examine the [pcap](https://gitlab.com/rdodin/pics/-/wikis/uploads/f2ab8e8ee7a5cd7f8f03a72528ca87bc/gnmic-tls-sros.pcapng) I captured with [containerlab wireshark integration](../manual/wireshark.md) that shows the flow of TCP handshake with TLS negotiation for the same gNMI Capabilities request. 

## Summary
Pretty neat, right? With just the two commands ([`tools cert ca create`](../cmd/tools/cert/ca/create.md) and [`tools cert sign`](../cmd/tools/cert/sign.md)) we managed to perform a lot of actions in the background which resulted in a signed CA and node certificates.

Those certificates we can now use for any protocol that requires TLS and the certificates are verifiable and legit.

[topofile]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/cert01/cert01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information. Memory deduplication techniques like [UKSM](https://netdevops.me/2021/how-to-patch-ubuntu-20.04-focal-fossa-with-uksm/) might help with RAM consumption.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
[^3]: Version of our fork - [hellt/vrnetlab](https://github.com/hellt/vrnetlab) with which the container image of this VM was generated.
[^4]: the `/etc/hosts` entry is created by containerlab when it deploys the nodes.