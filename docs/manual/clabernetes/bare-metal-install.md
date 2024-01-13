# Installing Clabernetes on a Bare Metal Cluster

## MetalLB

Enable strict ARP mode on the cluster:

TODO: check if kubeone can set it already

```
kubectl get configmap kube-proxy -n kube-system -o yaml | \
sed -e "s/strictARP: false/strictARP: true/" | \
kubectl apply -f - -n kube-system
```

Installing MetalLB:

```
helm repo add metallb https://metallb.github.io/metallb
helm install --create-namespace metallb metallb/metallb -n metallb-system
```

```yaml
kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: lb-pool
  namespace: metallb-system
spec:
  addresses:
  - 10.133.166.201/32
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: lb-pool
  namespace: metallb-system
EOF
```

## Traefik Load Balancer

Following the [Traefik documentation](https://doc.traefik.io/traefik/getting-started/install-traefik/#use-the-helm-chart) we install the latest traefik release on the cluster using Helm:

```bash
helm repo add traefik https://traefik.github.io/charts && \
helm repo update && \
helm upgrade --install --create-namespace --namespace traefik \
    traefik traefik/traefik
```

Traefik dashboard is not exposed by default. To enable it we use the following port-forwarding command:

```bash
kubectl --namespace traefik port-forward $(kubectl --namespace traefik get pods --selector "app.kubernetes.io/name=traefik" --output=name) 9000:9000
```
