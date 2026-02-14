# Topology Configuration

The Topology custom resource (CR) is the primary way to deploy containerlab topologies in Kubernetes. This page covers all available configuration options.

## Definition

The `definition` field contains the containerlab topology in YAML format:

```yaml
apiVersion: clabernetes.containerlab.dev/v1alpha1
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

Controls launcher pod settings.

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

Mount files from ConfigMaps or URLs into launcher pods.

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

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `deployment.privilegedLauncher` | bool | `true` | Run launcher in privileged mode |
| `deployment.containerlabDebug` | bool | `false` | Enable containerlab debug logging |
| `deployment.containerlabTimeout` | string | - | Deploy timeout (e.g., "30m") |
| `deployment.containerlabVersion` | string | - | Override containerlab version |
| `deployment.launcherImage` | string | - | Override launcher image |
| `deployment.launcherImagePullPolicy` | enum | - | Image pull policy: IfNotPresent, Always, Never |
| `deployment.launcherLogLevel` | enum | - | Log level: disabled, critical, warn, info, debug |
| `deployment.extraEnv` | list | - | Additional environment variables |

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

Control image pulling behavior:

```yaml
spec:
  imagePull:
    pullThroughOverride: auto
    insecureRegistries:
      - internal-registry.local:5000
    pullSecrets:
      - my-registry-secret
    dockerDaemonConfig: daemon-config-secret
    dockerConfig: docker-config-secret
```

| Field | Type | Description |
|-------|------|-------------|
| `pullThroughOverride` | enum | Pull-through mode: auto, always, never |
| `insecureRegistries` | list | Registries without valid TLS |
| `pullSecrets` | list | Kubernetes secrets for private registries |
| `dockerDaemonConfig` | string | Secret with daemon.json |
| `dockerConfig` | string | Secret with Docker config.json |

## Connectivity

Inter-node tunnel type for datapath stitching:

| Value | Description |
|-------|-------------|
| `vxlan` | VXLAN tunnels (default) |
| `slurpeeth` | Experimental TCP tunnels (avoids MTU issues) |

```yaml
spec:
  connectivity: vxlan
```

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
apiVersion: clabernetes.containerlab.dev/v1alpha1
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
    privilegedLauncher: true
    launcherLogLevel: info
  statusProbes:
    enabled: true
    probeConfiguration:
      startupSeconds: 900
      sshProbeConfiguration:
        username: admin
        password: NokiaSrl1!
  imagePull:
    pullThroughOverride: auto
  naming: prefixed
  connectivity: vxlan
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
