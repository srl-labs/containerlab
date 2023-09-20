---
status: new
---

# Clabernetes

<small>Clabernetes is in [:material-alpha: state](#whats-in-material-alpha-release) at the moment. We are trying (& breaking) things.</small>

Love containerlab? Want containerlab, just distributed in a kubernetes cluster? Enter [**clabernetes**](https://github.com/srl-labs/clabernetes/).

<figure markdown>
![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/4fdd35b5f4553d766216a4bda2b9a20c/geogebra-export.svg#only-light)
![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/a139e454c70614298f5bf5b86fe1eeb0/geogebra-export-darkbg.svg#only-dark)
</figure>

Clabernetes is a kubernetes controller that deploys valid containerlab topologies into a kubernetes cluster. The goal of Clabernetes is to scale Containerlab beyond a single node while keeping the same familiar user interface.

If all goes to plan, Clabernetes is going to be one of the solutions to enable [multi-node labs](../multi-node.md) and allow its users to create large topologies powered by a k8s cluster.

Eager to try it out? Check out the [Quickstart](quickstart.md)! Have questions, join our [Discord](https://discord.gg/2A8ZxM7hD9).

## What's in :material-alpha: release?

We are sharing Clabernetes in its early alpha stages to allow people to see what we're working on and potentially attract contributors and early adopters.

In the alpha release we focus on basic topology constructs working our way towards full feature parity with Containerlab. Here is what is supported from the topology definitions so far:

1. [startup-config](../nodes.md#startup-config) both inline and file-based formats.
2. [binds](../nodes.md#binds)
3. point to point links between the nodes.
4. automatic port exposure via Load Balancer, see [quickstart](quickstart.md#accessing-the-nodes).
5. custom ports exposure to expose ports which are not exposed by default.
6. Nodes requiring a license are not supported yet.
