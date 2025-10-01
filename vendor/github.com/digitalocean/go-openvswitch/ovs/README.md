ovs
===

Package `ovs` is a client library for Open vSwitch which enables programmatic
control of the virtual switch.

Package `ovs` is a wrapper around the `ovs-vsctl` and `ovs-ofctl` utilities, but
in the future, it may speak OVSDB and OpenFlow directly with the same interface.

```go
// Create a *ovs.Client.  Specify ovs.OptionFuncs to customize it.
c := ovs.New(
    // Prepend "sudo" to all commands.
    ovs.Sudo(),
)

// $ sudo ovs-vsctl --may-exist add-br ovsbr0
if err := c.VSwitch.AddBridge("ovsbr0"); err != nil {
    log.Fatalf("failed to add bridge: %v", err)
}

// $ sudo ovs-ofctl add-flow ovsbr0 priority=100,ip,actions=drop
err := c.OpenFlow.AddFlow("ovsbr0", &ovs.Flow{
    Priority: 100,
    Protocol: ovs.ProtocolIPv4,
    Actions:  []ovs.Action{ovs.Drop()},
})
if err != nil {
    log.Fatalf("failed to add flow: %v", err)
}
```
