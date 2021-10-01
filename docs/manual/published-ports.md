Labs are typically deployed in the isolated environments, such as company's internal network, cloud region or even a laptop. The lab nodes can happily talk to each other and, if needed, can reach Internet in the outbound direction.

But sometimes it is really needed to let your lab nodes be reachable over Internet securely and privately in the inbound direction. There are many use cases that warrant such _publishing_, some of the most common are:

* create a lab in your environment and share it with a customer/colleague on-demand
* make an interactive demo/training where nodes are shared with an audience for hands-on experience
* share a private lab with someone to collaborate or troubleshoot
* expose management interfaces (gNMI, NETCONF, SNMP) to test integration with collectors deployed outside of your lab environment

Check out the short video demonstrating the integration:

<iframe type="text/html"
    width="100%"
    height="480"
    src="https://www.youtube.com/embed/6t0fPJtwaGM"
    frameborder="0">
</iframe>

Containerlab made all of these use cases possible by integrating with [mysocket.io](https://mysocket.io) service. Mysocket.io provides personal and secure tunnels for https/https/tls/tcp ports over global anycast[^1] network spanning US, Europe and Asia.

To make a certain port of a node available via mysocket.io tunnel provide a `publish` container under the node/kind/default section of the topology:

```yaml
name: demo
topology:
  nodes:
    r1:
      kind: srl
      publish:
        # tcp port 22 will be published and accessible to anyone
        - tcp/22
        # tcp port 57400 will be published for a specific user only
        - tls/57400/user@domain.com
        # http service running over 10200 will be published
        # for any authenticated user within gmail domain
        - http/10200/gmail.com
```

<!-- <video width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/709405ded4ccf7387725b4fab1ab87f6/containerlab-mysocketio.mp4" type="video/mp4">
</video> -->

## Registration
Tunnels set up by mysocket.io are associated with a user who set them, thus users are required to register within the service. Luckily, the registration is a split second process carried out via a [web portal](https://portal.mysocket.io/register). All it takes is an email and a password.

## Acquiring a token
To authenticate with mysocket.io service a user needs to acquire the token by logging into the service. A helper command [`mysocketio login`](../cmd/tools/mysocketio/login.md) has been added to containerlab to help with that:

```bash
# Login with password entered from the prompt
containerlab tools mysocketio login -e myemail@dot.com
Password:
INFO[0000] Written mysocketio token to a file /root/containerlab/.mysocketio_token
```

The acquired token will be saved under `.mysocketio_token` filename in the current working directory.

!!!info
    The token is valid for 5 hours, once the token expires, the already established tunnels will continue to work, but to establish new tunnels a new token must be provided.

## Specify what to share
To indicate which ports to publish a users needs to add the `publish` section under the node/kind or default level of the [topology definition file](topo-def-file.md). In the example below, we are publishing SSH and gNMI services of `r1` node:

```yaml
name: demo
topology:
  nodes:
    r1:
      kind: srl
      publish:
        - tcp/22     # tcp port 22 will be exposed
        - tcp/57400  # tcp port 57400 will be exposed
```

The `publish` section holds a list of `<type>/<port-number>[/<allowed-domains-and-email>` strings, where

* `<type>` must be one of the supported mysocket.io socket type[^2] - http/https/tls/tcp
* `<port>` must be a single valid port value
* `<allowed-domains-and-email>` an optional element restricting access to published ports for a list of users' emails or domains. Read more about in the [Identity Aware tunnels](#identity-aware-tunnels) section.

!!!note
    For a regular mysocketio account the maximum number of tunnels is limited to:  
      - tcp based tunnels: 5  
      - http based tunnels: 10  
    If >5 tcp tunnels are required users should launch a VM in a lab, expose it's SSH service and use this VM as a jumpbox.

## Add mysocketio node
Containerlab integrates with mysocket.io service by leveraging `mysocketctl` application packaged in a [container](https://github.com/users/hellt/packages/container/package/mysocketctl) format. In order for the ports indicated in the `publish` block to be published, a user needs to add a `mysocketio` node to the topology. The complete topology file could look like this:

```yaml
name: publish
topology:
  nodes:
    r1:
      kind: srl
      image: ghcr.io/nokia/srlinux
      publish:
        - tcp/22     # tcp port 22 will be exposed

    grafana:
      kind: linux
      image: grafana/grafana:7.4.3
      publish:
        - http/3000  # grafana' default http port will be published

    # adding mysocketio container which has mysocketctl client inside
    mysocketio:
      kind: mysocketio
      image: ghcr.io/hellt/mysocketctl:0.5.0
      binds:
        - .mysocketio_token:/root/.mysocketio_token # bind mount API token
```

The `mysocketio` node is a tiny linux container with mysocketctl client installed. Containerlab uses this node to create the sockets and start the tunnels as per `publish` block instructions.

Pay specific attention to `binds` section defined for mysocketio node. With this section we provide a path to the API token that we acquired before launching the lab. This token is used to authenticate with mysocketio API service.

## Explore published ports
When a user launches a lab with published ports it will be presented with a summary table after the lab deployment process finishes:

```
+---+-----------------------+--------------+---------------------------------+------------+-------+---------+----------------+----------------------+
| # |         Name          | Container ID |              Image              |    Kind    | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------------------+--------------+---------------------------------+------------+-------+---------+----------------+----------------------+
| 1 | clab-sock-r1          | 9cefd6cdb239 | srlinux:20.6.3-145              | srl        |       | running | 172.20.20.2/24 | 2001:172:20:20::2/80 |
| 2 | clab-sock-mysocketctl | 8f5385beb97e | ghcr.io/hellt/mysocketctl:0.5.0 | mysocketio |       | running | 172.20.20.3/24 | 2001:172:20:20::3/80 |
+---+-----------------------+--------------+---------------------------------+------------+-------+---------+----------------+----------------------+
Published ports:
┌──────────────────────────────────────┬──────────────────────────────────────┬─────────┬──────┬────────────┬────────────────────────┐
│ SOCKET ID                            │ DNS NAME                             │ PORT(S) │ TYPE │ CLOUD AUTH │ NAME                   │
├──────────────────────────────────────┼──────────────────────────────────────┼─────────┼──────┼────────────┼────────────────────────┤
│ 444ed853-d3b6-448c-8f0a-6854b3578848 │ wild-water-9221.edge.mysocket.io     │ 80, 443 │ http │ false      │ clab-grafana-http-3000 │
│ 287e5962-29ac-4ca1-8e01-e0333d399070 │ falling-wave-5735.edge.mysocket.io   │ 54506   │ tcp  │ false      │ clab-r1-tcp-22         │
└──────────────────────────────────────┴──────────────────────────────────────┴─────────┴──────┴────────────┴────────────────────────┘
```
The **Published ports** table lists the published ports and their corresponding DNS names. Looking at the NAME column users can quickly discover which tunnel corresponds to which node-port. The socket name follows the `clab-<node-name>-<type>-<port>` pattern.

To access the published port, users need to combine the DNS name and the Port to derive the full address. For the exposed SSH port, for example, the ssh client can use the following command to access remote SSH service:

```
ssh user@falling-wave-5735.edge.mysocket.io -p 54506
```

!!!warning
    When a lab with published ports start, containerlab first removes all previously established tunnels. This means that any manually set up tunnels for this account will get removed.

## Identity aware tunnels
In the previous examples the published ports were created in a way that makes them accessible to anyone on the Internet who knows the exact domain name and port of a respective tunnel. Although being convenient, this approach is not secure, since there is no control over who can access the ports you published.

If additional security is needed, containerlab users should define the published ports using Identity Awareness[^3] feature of mysocketio. With Identity aware sockets users are allowed to specify a list of email addresses or domains which will have access to a certain port.

Consider the following snippet:

```yaml
topology:
  nodes:
    leaf1:
      publish:
        - tcp/22/dodin.roman@gmail.com,contractor@somewhere.com
    leaf2:
      publish:
        - tcp/22

    grafana:
      publish:
        - http/3000/gmail.com,nokia.com,colleague@somedomain.com
```

Within the same `publish` block it is possible to provide a list of comma separated emails and/or domains which will be allowed to access the published port in question.

Authentication is carried out by OAuth via Google, GitHub, Facebook and Mysocket.io providers. When accessing a secured tunnel, a browser page is opened asking to authenticate:

<p align=center>
<img src="https://gitlab.com/rdodin/pics/-/wikis/uploads/e5bdca34c5570d47cf1e1b73f77c5395/image.png" width="50%">
</p>

Once authenticated via any of the available providers the sessions will establish.

### TCP/TLS
With Identity Aware sockets used for SSH[^4] service, a client must have [mysocketctl](https://download.edge.mysocket.io/) client installed and use the following command to establish a connection:

```bash
ssh <ssh-username>@<mysocket-tunnel-address> \
    -o 'ProxyCommand=mysocketctl client tls --host %h'
```

As with HTTP services, a browser page will appear asking to proceed with authentication. Upon successful authentication, the SSH session will establish.

## Proxy
Mysocketio uses SSH as a dataplane to build tunnels, thus it needs to be able to have external SSH access towards the `ssh.mysocket.io` SSH server.

Chances are high that in your environment external SSH access might be blocked, preventing mysocket to setup tunnels. A possible solution for such environments would be to leverage the ability to tunnel SSH traffic via HTTP(S) proxies.

If your HTTP(S) proxy supports CONNECT method and is able to pass non-HTTP payloads, it is quite likely that mysocketio service will work.

To configure HTTP(S) proxy for mysocketio use the `mysocket-proxy` parameter in the `extras` section of the node definition:

```yaml
    mysocketio:
      kind: mysocketio
      image: ghcr.io/hellt/mysocketctl:0.5.0
      binds:
        - .mysocketio_token:/root/.mysocketio_token
      extras:
        mysocket-proxy: http://192.168.0.1:8000
```

## Troubleshooting
To check the health status of the established tunnels execute the following command to check the logs created on mysocketio container:

```
docker exec -it <mysocketio-node-name> /bin/sh -c "cat socket*"
```

This command will display all the logs for the published ports. If something is not right, you will see the errors in the log.

[^1]: https://mysocket.readthedocs.io/en/latest/about/about.html#build-on-a-global-anycast-network
[^2]: https://mysocket.readthedocs.io/en/latest/about/about.html#features
[^3]: Identity aware [HTTP](https://www.mysocket.io/post/introducing-identity-aware-sockets-enabling-zero-trust-access-for-your-private-services) and [TCP](https://www.mysocket.io/post/introducing-ssh-zero-trust-identity-aware-tcp-sockets) tunnels are available.
[^4]: [Read more](https://www.mysocket.io/post/introducing-ssh-zero-trust-identity-aware-tcp-sockets) about Identity Aware sockets for TCP in the official blog.