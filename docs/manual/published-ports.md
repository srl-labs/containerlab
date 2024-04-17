
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

Containerlab made all of these use cases possible by integrating with [border0.com](https://border0.com) service. border0.com provides personal and secure tunnels for https/https/tls/tcp/ssh ports over global anycast[^1] network spanning US, Europe and Asia.

To make a certain port of a node available via border0.com tunnel provide a `publish` container under the node/kind/default section of the topology:

```yaml
name: demo
topology:
  nodes:
    r1:
      kind: nokia_srlinux
      publish:
        # tcp port 22 will be published and accessible to anyone
        - tcp/22
        # tcp port 57400 will be published using a specific EXISTING border0.com policy
        - tls/57400/MyBorder0Policy
        # http service running over 10200 will be published
        # considering the two existing policies MyBorder0Policy and MyOtherPolicy
        - http/10200/MyBorder0Policy,MyOtherPolicy
```

<!-- <video width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/709405ded4ccf7387725b4fab1ab87f6/containerlab-mysocketio.mp4" type="video/mp4">
</video> -->

## Registration

Tunnels set up by border0.com are associated with a user who sets them, thus users are required to register within the service. Luckily, the registration is a split second process carried out via a [web portal](https://portal.border0.com/register). All it takes is an email and a password (or an existing google / github account).

## Acquiring a token

To authenticate with border0.com service a user needs to acquire the token by logging into the service. A helper command [`border0 login`](../cmd/tools/border0/login.md) has been added to containerlab to help with that:

```bash
# Login with password entered from the prompt
containerlab tools border0 login -e myemail@dot.com
Password:
INFO[0000] Written border0 token to a file /root/containerlab/.border0_token
```

The acquired token will be saved under `.border0_token` filename in the current working directory.

!!!info
    The token is valid for 5 hours, once the token expires, the already established tunnels will continue to work, but to establish new tunnels a new token must be provided.

## Specify what to share

To indicate which ports to publish a users needs to add the `publish` section under the node/kind or default level of the [topology definition file](topo-def-file.md). In the example below, we are publishing SSH and gNMI services of `r1` node:

```yaml
name: demo
topology:
  nodes:
    r1:
      kind: nokia_srlinux
      publish:
        - tls/22     # tcp port 22 will be exposed
        - tls/57400  # tcp port 57400 will be exposed
```

The `publish` section holds a list of `<type>/<port-number>[/<border0-policy>` strings, where

* `<type>` must be one of the supported border0.com socket type[^2] - http/https/tls/ssh
!!!note
    The ssh type is special in a way, that the border0.com service terminates the ssh connection to provide recording / replay capabilities etc. but therefore requires injection of an SSH-CA into the lab nodes, for the border0.com service to be able to establish the proxied ssh session to the containers. Therefore the tls kind should commonly be the right choise for the type.
* `<port>` must be a single valid port value
* `<border0-policy>` an optional element restricting access to published ports based on border0.com defined policies [border0.com policies](#border0com-policies) section.

!!!note
    For a regular border0.com account the maximum number of tunnels is limited to:  
      - tls based tunnels: 5
      - http based tunnels: 10  
    If >5 tcp tunnels are required users should launch a container / VM in a lab, expose it's SSH service and use it as a jumphost.

## Add border0.com node

Containerlab integrates with border0.com service by leveraging a [container](https://github.com/srl-labs/containerlab-border0.com) that has the border0.com client binary integrated. In order for the ports indicated in the `publish` block to be published, a user needs to add a `border0` node to the topology. The complete topology file could look like this:

```yaml
name: publish
topology:
  nodes:
    r1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
      publish:
        - tls/22     # tcp port 22 will be exposed

    grafana:
      kind: linux
      image: grafana/grafana:7.4.3
      publish:
        - http/3000  # grafana' default http port will be published

    # adding mysocketio container which has border0 client packaged
    border0:
      kind: border0
```

The `border0` node is a tiny linux container with border0 client installed. Containerlab uses this node to create the sockets and start the tunnels as per `publish` block instructions.

Internally containerlab utilizes the Static Sockets Plugin to provide the necessary configuration to the border0 process.

## Border0.com policies

Policies are used to control who has access to what Sockets and under what conditions. Think of policies as advanced, Identity-, application-aware, and context-aware firewall rules. Unlike traditional firewalls or access control list (ACL) rules, Border0 policies allow you to define access rules based on Identity, time of day, application type, and location. (see [https://docs.border0.com/docs/policies])

Within the `publish` block it is possible to provide a list of comma separated policy names which will be attached to the published port in question.

Authentication is carried out by OAuth via Google or GitHub providers.

Once authenticated via any of the available providers the sessions will establish.

### TCP/TLS

With Identity Aware sockets used for SSH[^4] service, a client must have [border0](https://docs.border0.com/docs/quick-start) client installed and use the following command to establish a connection:

```bash
ssh <ssh-username>@<mysocket-tunnel-address> \
    -o 'ProxyCommand=border0 client tls --host %h'
```

As with HTTP services, a browser page will appear asking to proceed with authentication. Upon successful authentication, the SSH session will establish.
