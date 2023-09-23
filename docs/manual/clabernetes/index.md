---
status: new
---

# Clabernetes

<small>Clabernetes is in [:material-alpha: state](#whats-in-release) at the moment. We are trying (& breaking) things.</small>

Love containerlab? Want containerlab, just distributed in a kubernetes cluster? Enter [**clabernetes**](https://github.com/srl-labs/clabernetes/) or simply **c9s**.

<figure markdown>
![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/4fdd35b5f4553d766216a4bda2b9a20c/geogebra-export.svg#only-light)
![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/a139e454c70614298f5bf5b86fe1eeb0/geogebra-export-darkbg.svg#only-dark)
</figure>

Clabernetes deploys containerlab topologies into a kubernetes cluster. The goal of Clabernetes is to scale Containerlab beyond a single node while keeping the user experience you love.

If all goes to plan, Clabernetes is going to be one of the solutions to enable [multi-node labs](../multi-node.md) and allow its users to create large topologies powered by a k8s cluster.

Eager to try it out? Check out the [Quickstart](quickstart.md)! Have questions, join our [Discord](https://discord.gg/2A8ZxM7hD9).

## What's in :material-alpha: release?

We are sharing Clabernetes in its early alpha stages to allow people to see what we're working on and potentially attract contributors and early adopters.

In the alpha release we focus on basic topology constructs working our way towards full feature parity with Containerlab. Here is what is supported from the topology definitions so far:

1. Images used in the topology should be available in the k8s cluster either by pulling them from a public registry or by using a private registry.
2. [startup-config](../nodes.md#startup-config) both inline and file-based formats.
3. [license](../nodes.md#license) provisioning.
4. point to point links between the nodes.
5. automatic port exposure via Load Balancer, see [quickstart](quickstart.md#accessing-the-nodes).
6. custom ports exposure to expose ports which are not exposed by default.

!!!question "Why not `openconfig/kne`"
    Clabernetes is an experiment to see if we can scale containerlab beyond a single node. Therefore, we wanted to keep containerlab core "as is" and not change the way users create topology files. We also wanted to offer the same user experience and more importantly the same set of supported Network OSes.

    [KNE](https://github.com/openconfig/kne) first and foremost focuses on the use cases of the Openconfig project, hence making it do what we need and want would not be feasible. With that in mind, we decided to simply take the best parts of containerlab and make it work in a kubernetes cluster.
