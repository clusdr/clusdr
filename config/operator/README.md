# clusdr-operator: same CLI in-cluster (init once, one --bootstrap, join).
#
# Public install (image is on the same tag as the daemon):
#   kubectl apply -f https://clusdr.io/download/clusdr-crds.yaml
#   kubectl apply -f https://clusdr.io/download/clusdr-operator.yaml
#   kubectl apply -f examples/k8s/clusdrcluster.yaml
#   # or sidecar (do not apply next to the DaemonSet CR):
#   # kubectl apply -f examples/k8s/clusdrcluster-sidecar.yaml
#
# GitHub fallback: releases/latest/download/clusdr-crds.yaml
# Contributor: kubectl apply -k config/crd && kubectl apply -k config/operator
# Local image: docker build -f Dockerfile.operator && kind load
#
# DaemonSet: hostPath uid 65532 still needs mkdir+chown on each node.
# Sidecar: PVC not emptyDir; bootstrap Job is deleted before the STS (RWO).
# App Local() is 127.0.0.1. Headless DNS is advertised node.addr, not ClusterIP.
# spec.leave is clusdr leave (member.left). A missing/restarted pod is not leave.
# Not injection. Helm chart stays DaemonSet.
