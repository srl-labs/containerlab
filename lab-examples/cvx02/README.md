Example of how to run `cvx` nodes inside docker runtime.

> **Note 1** : cvx may take up to a minute to configure IP on swp12. This is due to it defaulting to DHCP'ing first.
> **Node 2**: `h1` node requires `ifreload -a` before the IP is configured on its `eth1` interface