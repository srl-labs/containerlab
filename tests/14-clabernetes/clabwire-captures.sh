#!/bin/sh
# Prints the packet-capture processes running in the clabwire container of a c9s node's pod.
# usage: clabwire-captures.sh <namespace> <node>
#
# A process of a Kubernetes exec is not visible through the API, so this looks at the worker
# that runs the pod: a KinD node, which is a Docker container. It lists the processes whose
# cgroup belongs to the clabwire container. Exits 2 when the worker is not a Docker container.
set -eu

namespace=$1
node=$2

pod=$(kubectl -n "$namespace" get pod -l "c9s.run/topologyNode=$node" \
  -o jsonpath='{.items[0].metadata.name}')
worker=$(kubectl -n "$namespace" get pod "$pod" -o jsonpath='{.spec.nodeName}')
# clabwire is a sidecar (an init container that keeps running)
container=$(kubectl -n "$namespace" get pod "$pod" -o jsonpath=\
'{.status.initContainerStatuses[?(@.name=="clabwire")].containerID}{.status.containerStatuses[?(@.name=="clabwire")].containerID}')
container=${container#*://}

if ! docker inspect --type container "$worker" >/dev/null 2>&1; then
  echo "worker $worker of pod $namespace/$pod is not a Docker container (KinD)" >&2
  exit 2
fi
if [ -z "$container" ]; then
  echo "pod $namespace/$pod has no running clabwire container" >&2
  exit 1
fi

docker exec "$worker" sh -c "
  for proc in /proc/[0-9]*; do
    if grep -qs '$container' \"\$proc/cgroup\"; then
      tr '\\0' ' ' < \"\$proc/cmdline\" 2>/dev/null && echo
    fi
  done" | grep 'packet-capture' || true
