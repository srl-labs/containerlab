# Tailscale VPN Integration

Containerlab provides built-in support for [Tailscale](https://tailscale.com/) VPN to enable secure remote access to your lab's management network. This integration automatically deploys and configures a Tailscale container that connects your lab to your Tailscale network.

## Overview

When Tailscale is enabled, containerlab will:

1. Deploy a Tailscale container as infrastructure (not a lab node)
2. Advertise your management network routes to your Tailscale network
3. Configure the container with the appropriate IP address and labels
4. Manage the lifecycle of the Tailscale container alongside your lab

This allows you to access your lab nodes from anywhere, securely through Tailscale's encrypted mesh network.

## Prerequisites

Before using Tailscale with containerlab, you need:

1. **A Tailscale account** - Sign up at [https://login.tailscale.com/start](https://login.tailscale.com/start)
2. **An auth key** - Generate one in your [Tailscale admin console](https://login.tailscale.com/admin/settings/keys)
   - For labs, create a **reusable** auth key
   - Enable **route advertisement** in the key settings
   - Consider using **tagged keys** for better ACL control
   - Set an appropriate expiration time

## Quick Start

Add the `tailscale` section to your topology file:

```yaml
name: my-lab
mgmt:
  network: clab
  ipv4-subnet: 172.20.20.0/24
  tailscale:
    authkey: "tskey-auth-k1234567890CNTRL-abcdefghijklmnop"
```

Deploy your lab:

```bash
sudo containerlab deploy -t my-lab.clab.yml
```

Accept the advertised routes in your [Tailscale admin console](https://login.tailscale.com/admin/machines), then access your lab nodes:

```bash
# SSH to a node using its management IP
ssh admin@172.20.20.5

# Ping a node
ping 172.20.20.5
```

## Configuration Reference

### Minimal Configuration

```yaml
mgmt:
  tailscale:
    authkey: "tskey-auth-xxxxx"
```

### Full Configuration

```yaml
mgmt:
  network: clab
  ipv4-subnet: 172.20.20.0/24
  ipv6-subnet: 2001:172:20:20::/64
  tailscale:
    enabled: true
    authkey: "tskey-auth-xxxxx"
    image: "tailscale/tailscale:v1.56.0"
    ipv4-address: "172.20.20.254"
    ipv6-address: "2001:172:20:20::fffe"
    tags:
      - "lab-access"
      - "team:engineering"
    snat: false
    accept-routes: false
    accept-dns: false
    one-to-one-nat: "10.0.0.0/24"
    ephemeral-state: true
```

### Configuration Options

#### enabled

**Type:** `boolean`  
**Default:** `true` (when `tailscale` section exists)

Explicitly enable or disable Tailscale deployment. When the `tailscale` section is present, Tailscale is enabled by default.

```yaml
tailscale:
  enabled: false  # Disable even though section exists
  authkey: "tskey-auth-xxxxx"
```

#### authkey

**Type:** `string`  
**Required:** Yes

Your Tailscale authentication key. This is the only required field.

```yaml
tailscale:
  authkey: "tskey-auth-k1234567890CNTRL-abcdefghijklmnop"
```

!!! tip "Auth Key Best Practices"
    - Use **reusable** keys for labs that are deployed/destroyed frequently
    - Use **tagged** keys for better ACL control
    - Enable **route advertisement** in the key settings
    - Set reasonable expiration times
    - Store keys securely, consider using environment variables:
      ```yaml
      authkey: "${TAILSCALE_KEY}"
      ```

#### image

**Type:** `string`  
**Default:** `tailscale/tailscale:latest`

Docker image to use for the Tailscale container. Specify a version tag for reproducibility.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  image: "tailscale/tailscale:v1.56.0"
```

#### ipv4-address

**Type:** `string`  
**Default:** Last usable IP in the IPv4 subnet (excludes broadcast)

Custom IPv4 address for the Tailscale container.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  ipv4-address: "172.20.20.254"
```

!!! note
    The address must be within the management `ipv4-subnet` range.

#### ipv6-address

**Type:** `string`  
**Default:** Last IP in the IPv6 subnet

Custom IPv6 address for the Tailscale container.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  ipv6-address: "2001:172:20:20::fffe"
```

#### tags

**Type:** `list of strings`  
**Default:** `[]` (no tags)

Tailscale ACL tags to apply to the machine. Tags must be prefixed with `tag:` or the prefix will be added automatically.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  tags:
    - "lab"  # Will become "tag:lab"
    - "tag:production"  # Already prefixed
```

Use tags in your [Tailscale ACL](https://tailscale.com/kb/1018/acls/) to control access:

```json
{
  "tagOwners": {
    "tag:lab": ["group:developers"],
    "tag:production": ["group:admins"]
  },
  "acls": [
    {
      "action": "accept",
      "src": ["group:developers"],
      "dst": ["tag:lab:*"]
    }
  ]
}
```

#### snat

**Type:** `boolean`  
**Default:** `true`

Enable or disable source NAT for traffic originating from Tailscale.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  snat: false
```

#### accept-routes

**Type:** `boolean`  
**Default:** `false`

Accept subnet routes advertised by other nodes in your Tailscale network.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  accept-routes: true
```

#### accept-dns

**Type:** `boolean`  
**Default:** `false`

Accept DNS configuration from Tailscale MagicDNS.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  accept-dns: true
```

#### one-to-one-nat

**Type:** `string` (CIDR notation)  
**Default:** `""` (disabled)

Advertise a different subnet via Tailscale with 1:1 NAT mapping to the actual management network. This is useful when:

- The management subnet overlaps with other networks
- You want a stable IP range across lab redeployments
- You need to integrate with existing addressing schemes

```yaml
mgmt:
  ipv4-subnet: 172.20.20.0/24  # Actual management network
  tailscale:
    authkey: "tskey-auth-xxxxx"
    one-to-one-nat: "10.0.0.0/24"  # Advertised subnet
```

With this configuration:
- Node at `172.20.20.5` → accessible via `10.0.0.5`
- Node at `172.20.20.10` → accessible via `10.0.0.10`
- Tailscale container at `172.20.20.254` → accessible via `10.0.0.254`

**Requirements:**
- Source and destination subnets must be the same size
- Both must have the same prefix length
- Only IPv4 is supported for NAT

##### DNS Doctoring with NAT

When both NAT and DNS are enabled together, ContainerLab automatically sets up **DNS response rewriting** (DNS doctoring) to ensure DNS queries return the correct IP addresses based on the client's location:

```yaml
mgmt:
  ipv4-subnet: 172.20.20.0/24
  tailscale:
    authkey: "tskey-auth-xxxxx"
    one-to-one-nat: "172.20.200.0/24"
    dns:
      enabled: true
```

**How it works:**

1. A lightweight DNS proxy sits in front of CoreDNS
2. Queries from **Tailscale clients** (100.x.x.x IPs) get responses with **NAT IPs**
3. Queries from **local network** get responses with **real management IPs**

**Example:**

```bash
# From Tailscale client
$ nslookup node1.mylab.clab 172.20.200.254
Address: 172.20.200.5  # ← NAT IP (translated)

# From lab host (local network)
$ nslookup node1.mylab.clab 172.20.20.254
Address: 172.20.20.5   # ← Real management IP
```

**Architecture:**

```
Tailscale Client → DNS Proxy (port 53) → CoreDNS (port 5353)
                       ↓
                  Rewrites IPs based on client source:
                  - From 100.x.x.x → return NAT IPs
                  - From local net → return real IPs
```

The DNS proxy automatically:
- Detects query source (Tailscale vs local)
- Translates A record responses for Tailscale clients
- Preserves IPv6 addresses unchanged
- Works transparently with MagicDNS

**Note:** DNS doctoring is only active when **both** `one-to-one-nat` **and** `dns.enabled` are configured. With NAT only (no DNS), you access nodes directly via NAT IPs.

#### ephemeral-state

**Type:** `boolean`  
**Default:** `false`

Use ephemeral/in-memory state instead of persisting Tailscale state to disk. When enabled:

- Tailscale runs with `TS_STATE_DIR=mem:`
- Device is automatically removed from your Tailscale network when the container stops
- No persistent state is stored in `/var/lib/tailscale`

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  ephemeral-state: true
```

**When to use:**
- Temporary or frequently recreated labs
- CI/CD environments
- Testing scenarios
- When you don't want devices accumulating in your Tailscale admin console

**When NOT to use:**
- Production labs that should persist in your Tailscale network
- When you need to preserve Tailscale state across container restarts
- When debugging requires consistent device identity

!!! tip "Auth Keys with Ephemeral State"
    When using `ephemeral-state: true`, make sure your auth key is:
    
    - **Reusable** - so the same key can be used each time the lab is deployed

## Use Cases

### Remote Lab Access

Enable remote team members to access lab infrastructure securely:

```yaml
name: team-lab
mgmt:
  network: clab
  ipv4-subnet: 172.20.20.0/24
  tailscale:
    authkey: "tskey-auth-xxxxx"
    tags:
      - "team-shared"
```

### Multi-Site Labs

Connect labs across different locations or cloud providers:

```yaml
name: site-a
mgmt:
  tailscale:
    authkey: "tskey-auth-xxxxx"
    one-to-one-nat: "10.100.0.0/24"
    tags:
      - "site-a"

---
name: site-b
mgmt:
  tailscale:
    authkey: "tskey-auth-xxxxx"
    one-to-one-nat: "10.200.0.0/24"
    tags:
      - "site-b"
```

### Development and CI/CD

Provide developers secure access to ephemeral test environments:

```yaml
name: ci-test-${BUILD_ID}
mgmt:
  tailscale:
    authkey: "${TAILSCALE_CI_KEY}"
    tags:
      - "ci"
      - "temporary"
```

## MagicDNS and Split DNS

Containerlab supports running a DNS server inside the Tailscale container to enable DNS resolution of lab nodes using FQDNs like `node-name.clab`. This integrates with Tailscale's MagicDNS feature to provide seamless name resolution across your tailnet.

### How It Works

When DNS is enabled:

1. **CoreDNS runs in the Tailscale container** - A DNS server is automatically installed and configured
2. **Node records are automatically generated** - After lab deployment, DNS records are created for all nodes
3. **Split DNS via Tailscale MagicDNS** - Configure Tailscale to use the containerlab DNS for `.<lab-name>.clab` domains
4. **FQDN access** - Access nodes using `<shortname>.<lab-name>.clab` from anywhere on your tailnet

### Configuration

#### Step 1: Enable DNS in Containerlab

Add the `dns` section to your Tailscale configuration:

```yaml
name: my-lab
mgmt:
  network: clab
  ipv4-subnet: 172.20.20.0/24
  tailscale:
    authkey: "tskey-auth-xxxxx"
    dns:
      enabled: true
      domain: "my-lab.clab"     # optional, defaults to "<lab-name>.clab"
      port: 53                  # optional, defaults to 53
      coredns-version: "1.13.1" # optional, defaults to "1.13.1"
```

#### Step 2: Configure Tailscale MagicDNS

1. **Enable MagicDNS** in your [Tailscale admin console](https://login.tailscale.com/admin/dns):
   - Navigate to DNS settings
   - Enable "MagicDNS"

2. **Add Split DNS nameserver**:
   - Under "Nameservers" → "Split DNS"
   - Add a nameserver entry:
     - **Domain suffix**: `my-lab.clab` (or your custom domain)
     - **Nameservers**: The Tailscale container's IP address
       - **Without 1:1 NAT**: Use the real management IP (e.g., `172.20.20.254`)
       - **With 1:1 NAT**: Use the NAT IP (e.g., `172.20.200.254` or `10.0.0.254`)

   **Important for NAT users:** If you configured `one-to-one-nat`, use the **NAT IP** in MagicDNS settings, not the real management IP. The DNS proxy will automatically return NAT-translated IPs to Tailscale clients.

3. **Deploy your lab**:
   ```bash
   sudo containerlab deploy -t my-lab.clab.yml
```#### Step 3: Test DNS Resolution

From any device connected to your Tailscale network:

```bash
# Resolve a node by its short name (assuming lab name is "my-lab")
ping node1.my-lab.clab

# SSH using FQDN
ssh admin@router1.my-lab.clab

# Works with both IPv4 and IPv6
ping6 switch1.my-lab.clab
```

### DNS Configuration Options

#### dns.enabled

**Type:** `boolean`  
**Default:** `false`

Enable DNS server in the Tailscale container.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  dns:
    enabled: true
```

#### dns.domain

**Type:** `string`  
**Default:** `<lab-name>.clab`

DNS domain suffix for containerlab nodes. Nodes will be accessible as `<node-name>.<domain>`.

By default, the domain is `<lab-name>.clab`. For a lab named `my-lab`, nodes will be `node1.my-lab.clab`.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  dns:
    enabled: true
    domain: "mylab.local"  # Override default - nodes accessible as node1.mylab.local
```

#### dns.port

**Type:** `integer`  
**Default:** `53`

DNS server listen port. Usually doesn't need to be changed.

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  dns:
    enabled: true
    port: 5353  # custom port
```

#### dns.coredns-version

**Type:** `string`  
**Default:** `1.13.1`

CoreDNS version to install in the Tailscale container. Specify a version number (without the 'v' prefix).

```yaml
tailscale:
  authkey: "tskey-auth-xxxxx"
  dns:
    enabled: true
    coredns-version: "1.13.1"  # or any other CoreDNS version
```

### DNS Record Format

Containerlab automatically generates DNS records for all nodes:

- **Format**: `<shortname>.<lab-name>.clab` (or `<shortname>.<custom-domain>` if domain is overridden)
- **Example**: For a lab named `my-lab` with a node shortname `router1`, the FQDN is `router1.my-lab.clab`
- **Both IPv4 and IPv6**: If a node has both IPv4 and IPv6 management addresses, both A and AAAA records are created

**Example DNS Records (lab name: `my-lab`):**

| Node ShortName | Management IPv4 | Management IPv6 | FQDN |
|----------------|----------------|----------------|------|
| router1 | 172.20.20.10 | 3fff:172:20:20::10 | router1.my-lab.clab |
| switch1 | 172.20.20.11 | 3fff:172:20:20::11 | switch1.my-lab.clab |
| server1 | 172.20.20.12 | - | server1.my-lab.clab |

### Complete Example

```yaml
name: dns-lab
mgmt:
  network: clab
  ipv4-subnet: 172.20.20.0/24
  ipv6-subnet: 3fff:172:20:20::/64
  tailscale:
    authkey: "tskey-auth-xxxxx"
    tags:
      - "lab-dns"
    dns:
      enabled: true
      domain: "clab"

topology:
  nodes:
    router1:
      kind: nokia_srlinux
      type: ixrd3
    router2:
      kind: nokia_srlinux
      type: ixrd3
    switch1:
      kind: nokia_srlinux
      type: ixrd2
  links:
    - endpoints: ["router1:e1-1", "router2:e1-1"]
    - endpoints: ["router1:e1-2", "switch1:e1-1"]
```

After deployment:

```bash
# From your laptop (connected to Tailscale)
$ ping router1.dns-lab.clab
PING router1.dns-lab.clab (172.20.20.10): 56 data bytes
64 bytes from 172.20.20.10: icmp_seq=0 ttl=64 time=45.2 ms

$ ssh admin@switch1.dns-lab.clab
Warning: Permanently added 'switch1.dns-lab.clab,172.20.20.12' to the list of known hosts.
admin@switch1:~$
```

### Troubleshooting DNS

#### Check DNS Server Status

```bash
# Check if CoreDNS is running
docker exec clab-mylab-tailscale ps aux | grep coredns

# View CoreDNS logs
docker logs clab-mylab-tailscale 2>&1 | grep "coredns:"

# Check Corefile configuration
docker exec clab-mylab-tailscale cat /etc/coredns/Corefile

# View generated DNS records
docker exec clab-mylab-tailscale cat /etc/coredns/hosts
```

#### Test DNS Resolution from Tailscale Container

```bash
# Test DNS lookup from inside the container (assuming lab name is "my-lab")
docker exec clab-my-lab-tailscale nslookup router1.my-lab.clab localhost
```

#### Common Issues

**DNS not resolving:**
- Verify MagicDNS is enabled in Tailscale admin console
- Check that split DNS is configured with the correct domain suffix (`<lab-name>.clab`)
- Ensure the nameserver IP matches your Tailscale container's management IP
- Verify CoreDNS is running: `docker exec clab-mylab-tailscale ps aux | grep coredns`

**Stale DNS records:**
- DNS records are generated after all nodes are deployed
- If you add nodes dynamically, you may need to redeploy the lab or manually update DNS

**Wrong domain:**
- Check the `dns.domain` setting in your topology file (defaults to `<lab-name>.clab`)
- Verify split DNS in Tailscale uses the same domain suffix
- Remember: if your lab is named "production", the default domain is "production.clab"

## Operations

### Checking Status

View the Tailscale container:

```bash
docker ps -f name=tailscale
```

Check Tailscale connectivity:

```bash
# View full Tailscale status
docker exec clab-mylab-tailscale tailscale status

# Check advertised routes
docker exec clab-mylab-tailscale tailscale status --json | jq '.Self.AllowedIPs'

# View Tailscale logs
docker logs clab-mylab-tailscale
```

### Route Management

After deployment, you need to approve the advertised routes:

1. Go to [Tailscale admin console](https://login.tailscale.com/admin/machines)
2. Find your lab's machine (hostname: `clab-<prefix>-<labname>-tailscale`)
3. Click on the machine
4. Approve the advertised routes

Or use auto-approval with tagged machines in your ACL:

```json
{
  "autoApprovers": {
    "routes": {
      "10.0.0.0/8": ["tag:lab"],
      "172.20.0.0/16": ["tag:lab"]
    }
  }
}
```

### Troubleshooting

#### Routes Not Appearing

**Problem:** Tailscale routes don't show up in admin console

**Solution:**
- Ensure route advertisement is enabled in your auth key settings
- Check Tailscale logs: `docker logs clab-mylab-tailscale`
- Verify the container is running: `docker ps -f name=tailscale`

#### Can't Connect to Lab Nodes

**Problem:** Unable to reach lab nodes via Tailscale

**Solutions:**
1. Verify routes are approved in Tailscale admin console
2. Check your client is connected to Tailscale: `tailscale status`
3. Verify the routes are present on your client: `ip route` (Linux) or `netstat -rn` (macOS)
4. Test connectivity to the Tailscale container first: `ping <tailscale-container-ip>`

#### 1:1 NAT Not Working

**Problem:** NAT translation not functioning

**Solutions:**
1. Check iptables rules are installed:
   ```bash
   docker exec clab-mylab-tailscale iptables -t nat -L PREROUTING -n -v
   docker exec clab-mylab-tailscale iptables -t nat -L POSTROUTING -n -v
   ```
2. Verify kernel modules: `lsmod | grep iptable_nat`
3. Check subnet sizes match exactly

#### Auth Key Expired

**Problem:** Tailscale container fails to authenticate

**Solution:**
- Generate a new auth key in Tailscale admin console
- Update your topology file
- Redeploy: `sudo containerlab deploy -t mylab.clab.yml --reconfigure`

## Security Best Practices

### Auth Key Management

1. **Use reusable keys** for frequently deployed/destroyed labs
2. **Set expiration dates** appropriate for your use case
3. **Use tagged keys** with ACL restrictions
4. **Store keys securely**, never commit to version control
5. **Use environment variables** for auth keys in topology files

### Network Segmentation

Use Tailscale ACLs to segment access:

```json
{
  "tagOwners": {
    "tag:lab-dev": ["group:developers"],
    "tag:lab-prod": ["group:admins"]
  },
  "acls": [
    {
      "action": "accept",
      "src": ["group:developers"],
      "dst": ["tag:lab-dev:*"]
    },
    {
      "action": "accept",
      "src": ["group:admins"],
      "dst": ["tag:lab-prod:*", "tag:lab-dev:*"]
    }
  ]
}
```

### Monitoring

Set up [Tailscale logging](https://tailscale.com/kb/1255/network-flow-logs/) to track access to your labs.

## Lifecycle Management

The Tailscale container is managed automatically by containerlab:

| Event | Action |
|-------|--------|
| **Lab Deploy** | Tailscale container is created after the management network |
| **Lab Destroy** | Tailscale container is removed |
| **Lab Redeploy** | Tailscale container is reused if management network persists |
| **Network Recreate** | Tailscale container is recreated |

The Tailscale container is labeled with `clab-is-infrastructure: true` to distinguish it from regular lab nodes.

## Limitations

1. **Docker only** - Tailscale integration currently only supports Docker runtime
2. **IPv4 NAT only** - 1:1 NAT feature only supports IPv4
3. **Single Tailscale container** - One per lab (uses management network)
4. **Auth key required** - No support for interactive authentication

## Examples

See the [network documentation](network.md#tailscale-vpn) for complete examples and integration with other management network features.

## Reference

- [Tailscale Documentation](https://tailscale.com/kb/)
- [Tailscale ACLs](https://tailscale.com/kb/1018/acls/)
- [Tailscale Auth Keys](https://tailscale.com/kb/1085/auth-keys/)
- [Subnet Routers](https://tailscale.com/kb/1019/subnets/)
