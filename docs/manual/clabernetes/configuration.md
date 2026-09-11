# Topology Configuration

The Topology custom resource (CR) is the high-level API for deploying a complete containerlab definition. c9s compiles it into the primary Node, Link, and NodeProfile resources. This page covers the Topology configuration options.

## Definition

The `definition` field contains the containerlab topology in YAML format:

```yaml
apiVersion: c9s.run/v1alpha1
kind: Topology
metadata:
  name: my-lab
spec:
  definition:
    containerlab: |
      name: my-lab
      topology:
        nodes:
          srl1:
            kind: nokia_srlinux
            image: ghcr.io/nokia/srlinux:latest
```

## Expose Configuration

Controls how topology nodes are exposed via Kubernetes Services.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `expose.exposeType` | enum | `LoadBalancer` | Service type: `LoadBalancer`, `ClusterIP`, `Headless`, or `None` |
| `expose.disableExpose` | bool | `false` | Disable all service creation |
| `expose.disableAutoExpose` | bool | `false` | Only expose ports explicitly defined in topology |

### Expose Types

- **LoadBalancer**: External IP via cloud load balancer or MetalLB
- **ClusterIP**: Internal cluster access only
- **Headless**: Direct pod DNS resolution (no kube-proxy)
- **None**: No services created

### Auto-Exposed Ports

When `disableAutoExpose: false` (default), the following ports are automatically exposed:

| Protocol | Ports |
|----------|-------|
| TCP | 21, 22, 23, 80, 443, 830, 5000, 5900, 6030, 9339, 9340, 9559, 57400 |
| UDP | 161 |

### Example

```yaml
spec:
  expose:
    exposeType: ClusterIP
    disableAutoExpose: true
```

## Deployment Configuration

Controls the Kubernetes side of the device workloads.

### Resources

Per-node CPU and memory requirements:

```yaml
spec:
  deployment:
    resources:
      default:  # Applied to all nodes
        requests:
          memory: "2Gi"
          cpu: "1"
        limits:
          memory: "4Gi"
          cpu: "2"
      srl1:  # Override for specific node
        requests:
          memory: "4Gi"
          cpu: "2"
```

### Scheduling

Node placement constraints using node selectors and tolerations:

```yaml
spec:
  deployment:
    scheduling:
      nodeSelector:
        kubernetes.io/arch: amd64
        node-type: network-lab
      tolerations:
        - key: "dedicated"
          operator: "Equal"
          value: "network-lab"
          effect: "NoSchedule"
```

### File Mounting

Mount files from ConfigMaps, Secrets, or URLs into the device pods.

#### From ConfigMap

```yaml
spec:
  deployment:
    filesFromConfigMap:
      srl1:
        - filePath: /opt/srlinux/etc/license.key
          configMapName: srl-license
          configMapPath: license.key
          mode: read  # or "execute"
```

#### From URL

```yaml
spec:
  deployment:
    filesFromURL:
      srl1:
        - filePath: /tmp/config.json
          url: https://example.com/config.json
```

### Persistence

Enable persistent storage across pod restarts:

```yaml
spec:
  deployment:
    persistence:
      enabled: true
      claimSize: "10Gi"
      storageClassName: "fast-ssd"
```

/// note
PVC size cannot be reduced after creation. Storage class is immutable after creation.
///

### Other Deployment Options

The device image is pulled natively by the kubelet; use `imagePull.policy` and
`imagePull.pullSecrets` to control it.

## Status Probes

Health checking for containerlab nodes using SSH or TCP probes.

```yaml
spec:
  statusProbes:
    enabled: true
    excludedNodes:
      - linux-host
    probeConfiguration:
      startupSeconds: 900
      sshProbeConfiguration:
        username: admin
        password: NokiaSrl1!
        port: 22
    nodeProbeConfigurations:
      router1:
        tcpProbeConfiguration:
          port: 830
```

| Field | Description |
|-------|-------------|
| `enabled` | Enable/disable status probes |
| `excludedNodes` | Nodes to exclude from probing |
| `probeConfiguration` | Default probe config for all nodes |
| `nodeProbeConfigurations` | Per-node probe overrides |
| `startupSeconds` | Total seconds allowed for node startup |

## Image Pull Configuration

The kubelet pulls device images natively:

```yaml
spec:
  imagePull:
    policy: IfNotPresent
    pullSecrets:
      - my-registry-secret
```

| Field | Type | Description |
|-------|------|-------------|
| `policy` | enum | Kubernetes pull policy: IfNotPresent, Always, Never |
| `pullSecrets` | list | Same-namespace dockerconfigjson Secrets handed to the kubelet |

The manager also uses the pull secrets to read image metadata from the
registry when planning a device workload.

## Naming

Resource naming convention (immutable after creation):

| Value | Description |
|-------|-------------|
| `prefixed` | Include topology name in resource names |
| `non-prefixed` | No topology prefix (use separate namespaces) |
| `global` | Defer to global Config CRD |

```yaml
spec:
  naming: prefixed
```

## Complete Example

```yaml
apiVersion: c9s.run/v1alpha1
kind: Topology
metadata:
  name: production-lab
spec:
  expose:
    exposeType: LoadBalancer
    disableAutoExpose: false
  deployment:
    resources:
      default:
        requests:
          memory: "4Gi"
          cpu: "2"
      core-router:
        requests:
          memory: "16Gi"
          cpu: "8"
    scheduling:
      nodeSelector:
        node-type: network-lab
    persistence:
      enabled: true
      claimSize: "20Gi"
    filesFromConfigMap:
      srl1:
        - filePath: /opt/srlinux/etc/license.key
          configMapName: srl-license
          configMapPath: license.key
  statusProbes:
    enabled: true
    probeConfiguration:
      startupSeconds: 900
      sshProbeConfiguration:
        username: admin
        password: NokiaSrl1!
  imagePull:
    policy: IfNotPresent
  naming: prefixed
  definition:
    containerlab: |
      name: production
      topology:
        nodes:
          core-router:
            kind: nokia_srlinux
            image: ghcr.io/nokia/srlinux:latest
          edge1:
            kind: nokia_srlinux
            image: ghcr.io/nokia/srlinux:latest
        links:
          - endpoints: ["core-router:e1-1", "edge1:e1-1"]
```

## Nokia SR-SIM Support

Clabernetes supports [Nokia SR-SIM](../kinds/sros.md) deployments, including distributed chassis systems (SR-7, SR-14s, etc.). For distributed systems using `network-mode: container:<primary>`, clabernetes automatically groups all cards into a single pod.
