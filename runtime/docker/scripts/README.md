# ContainerLab Tailscale Scripts

This directory contains embedded scripts used by ContainerLab's Tailscale integration feature. These scripts are embedded into the ContainerLab binary at compile time using Go's `//go:embed` directive.

## Scripts Overview

### 1. `nat-setup.sh`
**Purpose**: Configures iptables rules for 1:1 NAT translation between management and NAT subnets.

**When Used**: Injected into Tailscale container startup command when `one-to-one-nat` is configured.

**Template Variables**:
- `{{.MgmtSubnet}}` - Real management subnet (e.g., `172.20.20.0/24`)
- `{{.NatSubnet}}` - NAT subnet advertised to Tailscale (e.g., `172.20.200.0/24`)

**What It Does**:
1. Starts containerboot in background
2. Waits 2 seconds for Tailscale to initialize
3. Applies DNAT rules (incoming traffic: NAT subnet → management subnet)
4. Applies SNAT rules (outgoing traffic via tailscale0: management → NAT subnet)
5. Adds FORWARD rules to allow traffic between subnets
6. Keeps container running by waiting for containerboot

**Why Embedded**: Ensures NAT rules persist across container restarts by being part of the container's startup command rather than applied post-deployment.

---

### 2. `dns-proxy.py`
**Purpose**: DNS proxy that rewrites DNS queries and responses for bidirectional NAT translation.

**When Used**: Started when both `dns.enabled` and `one-to-one-nat` are configured (DNS doctoring).

**Template Variables**:
- `{{.ListenPort}}` - Port to listen on (default: 53)
- `{{.BackendPort}}` - CoreDNS backend port (default: 5353)
- `{{.MgmtSubnet}}` - Real management subnet
- `{{.NatSubnet}}` - NAT subnet

**What It Does**:
1. Listens for DNS queries on the specified port
2. Detects if query is from Tailscale (100.x.x.x or fd7a: IP range)
3. **PTR Query Rewriting** (for Tailscale clients):
   - Rewrites PTR queries for NAT IPs to real IPs before forwarding
   - Example: Query for `254.200.20.172.in-addr.arpa` → `254.20.20.172.in-addr.arpa`
4. Forwards query to CoreDNS backend
5. **A Record Response Rewriting** (for Tailscale clients):
   - Rewrites A records from management IPs to NAT IPs in responses
   - Example: Response with `172.20.20.11` → `172.20.200.11`
6. For local clients: No rewriting, returns original IPs

**Bidirectional Translation**:
```
PTR Queries (Request Rewriting):
Tailscale Client queries 172.20.200.11
  → Proxy rewrites query to 172.20.20.11
  → CoreDNS resolves hostname
  → Response returned with hostname (no IP translation needed)

A Queries (Response Rewriting):
Tailscale Client queries hostname
  → Proxy forwards query unchanged
  → CoreDNS returns 172.20.20.11
  → Proxy rewrites response to 172.20.200.11
  → Client receives NAT IP
```

**Architecture**:
```
Tailscale Client (100.x.x.x) → DNS Proxy (port 53) → CoreDNS (port 5353)
                               ↓ PTR query rewrite
                               ↓ A response rewrite
                            NAT IP returned/queried

Local Client → DNS Proxy (port 53) → CoreDNS (port 5353)
                    ↓ (no rewrite)
               Real IP returned/queried
```

**Why Python**: Simple UDP socket handling and DNS packet manipulation without external dependencies.

---

### 3. `coredns-install.sh`
**Purpose**: Installs CoreDNS binary and optionally Python3 into the Tailscale container.

**When Used**: During Tailscale DNS setup when `dns.enabled` is true.

**Template Variables**:
- `{{.CoreDNSVersion}}` - CoreDNS version to install (default: 1.13.1)
- `{{.NeedsPython}}` - Boolean string ("true"/"false") indicating if Python is needed

**What It Does**:
1. Updates Alpine package manager
2. Installs wget (always required)
3. Installs Python3 (only if NAT is enabled for dns-proxy.py)
4. Downloads CoreDNS release tarball from GitHub
5. Extracts binary to `/usr/local/bin/coredns`
6. Creates `/etc/coredns` directory for configuration

**Why Embedded**: Single installation script that handles all dependencies based on configuration.

---

### 4. `Corefile.tmpl`
**Purpose**: CoreDNS configuration template.

**When Used**: Generated during DNS setup and updated when DNS records change.

**Template Variables**:
- `{{.LabName}}` - Lab name for comments
- `{{.Domain}}` - DNS domain to serve (e.g., `cisco-test.clab`)

**What It Does**:
Configures CoreDNS with two zones:
1. **Lab domain zone**: Serves lab-specific DNS records from `/etc/coredns/hosts`
2. **Catch-all zone (.)**: Forwards all other queries to system resolver

**Features**:
- Query logging with detailed format
- Error logging
- Fallthrough from hosts to forwarding
- PTR record forwarding for reverse DNS

**Port Configuration**: Port is specified via `-dns.port` flag at runtime, not in the Corefile.

---

## How Scripts Are Used

### Build Time
Scripts are embedded into the ContainerLab binary using Go's `embed` package:

```go
//go:embed scripts/nat-setup.sh
var natSetupScript string
```

### Runtime
1. Script templates are parsed using Go's `text/template` package
2. Variables are substituted based on lab configuration
3. Rendered scripts are executed in or written to the Tailscale container

## Modifying Scripts

### Development Workflow
1. Edit script files in this directory
2. Rebuild ContainerLab (`make build`)
3. Scripts are automatically embedded in the new binary
4. Test with a lab deployment

### Template Syntax
Scripts use Go template syntax for variable substitution:
- `{{.VariableName}}` - Simple variable substitution
- Variables must match struct field names in Go code


## Troubleshooting

### View Script Execution
Scripts output is visible in container logs:
```bash
docker logs clab-<lab>-tailscale
```

### Common Issues

**NAT rules not applied**:
- Check `nat-setup.sh` template variables are correct
- Verify Tailscale has NET_ADMIN capability
- Check container logs for iptables errors

**DNS proxy not rewriting**:
- Verify client source IP detection (100.x.x.x for Tailscale)
- Check subnet configuration matches NAT setup
- Review dns-proxy logs for rewrite messages

**CoreDNS installation fails**:
- Check internet connectivity from container
- Verify CoreDNS version exists on GitHub releases
- Check available disk space

## Architecture Benefits

### Single Binary Distribution
- No external script files to manage
- Scripts versioned with code
- Deployment simplified

### Template-Based Configuration
- Type-safe variable substitution
- Compile-time validation
- Runtime flexibility

### Maintainability
- Scripts in separate files with proper syntax highlighting
- Easy to test and modify
- Clear separation of concerns

## Related Documentation

- Main Tailscale documentation: `docs/manual/tailscale.md`
- Go embed documentation: https://pkg.go.dev/embed
- CoreDNS documentation: https://coredns.io/manual/toc/
- Tailscale documentation: https://tailscale.com/kb/

## Version History

- **2025-11-10**: Initial refactoring to go:embed
  - Extracted ~260 lines of embedded strings to separate files
  - Added template-based configuration
  - Improved maintainability
