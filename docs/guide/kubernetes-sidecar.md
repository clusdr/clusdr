# Run a sidecar StatefulSet

Use this only when the **replica is the Raft member**. Shared netns → `127.0.0.1` / `Local()`. N replicas = N members.

Default remains [one daemon per node](kubernetes.md). Do not apply this next to the DaemonSet example unless you mean **two** clusters. Why this is the exception: [Kubernetes](../concepts/kubernetes.md).

The published image has **no shell**. One pod template cannot mix seed `start --bootstrap` and joiner `start`. Use a Job on PVC-0, delete that Job (RWO), then start the StatefulSet. The [Operator](kubernetes-operator.md) does that sequence when `spec.topology` is `Sidecar`.

## 1. Bootstrap ordinal 0

```bash
kubectl apply -f examples/k8s/sidecar-bootstrap.yaml
```

Wait for the Job to finish. Copy the join token if you will join by hand. Delete the Job so the StatefulSet can mount the RWO volume.

Manifest: [`sidecar-bootstrap.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/sidecar-bootstrap.yaml).

## 2. Start the StatefulSet

```bash
kubectl apply -f examples/k8s/sidecar-statefulset.yaml
```

Headless Service DNS is the advertised `node.addr` / `raft.addr` (`$(POD_NAME).svc:7947`). The sidecar binds `0.0.0.0:7947`. The app in that pod dials `127.0.0.1:7947`.

## 3. Join ordinals ≥ 1

Same token as on a VM, seed Runtime = ordinal 0’s headless name. Or apply the Sidecar `ClusdrCluster` and let the Operator join.

## Checkpoint

Three pods, three `alive` members. The app in the pod uses `Local()` with no `CLUSDR_GRPC_ADDR`. Scaling the StatefulSet scales Raft. Do not put a ClusterIP in front and `Dial` it from every replica.

Walkthrough notes: [examples/k8s README](https://github.com/clusdr/clusdr/blob/main/examples/k8s/README.md#sidecar-statefulset-exception). The Helm chart does not install this topology.
