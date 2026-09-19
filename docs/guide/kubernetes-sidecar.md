# Run a sidecar StatefulSet

Use a sidecar only when the **replica is the Raft member**: a small elected StatefulSet, app + clusdr in one pod, `Local()` on `127.0.0.1`. N replicas = N members. Scaling this StatefulSet scales Raft.

Default remains [one daemon per node](kubernetes.md). Do not apply this next to the DaemonSet example unless you mean **two** clusters. Why this is the exception: [Kubernetes](../concepts/kubernetes.md).

The published image has **no shell**. One pod template cannot mix seed `start --bootstrap` and joiner `start`. Use a Job on PVC-0, delete that Job (RWO volumes allow one mount), then start the StatefulSet. The [Operator](kubernetes-operator.md) does that sequence when `spec.topology` is `Sidecar`.

Run the commands from a checkout that contains `examples/k8s/`.

## 1. Bootstrap ordinal 0

```bash
kubectl apply -f examples/k8s/sidecar-bootstrap.yaml
kubectl wait --for=condition=complete job/clusdr-sidecar-bootstrap --timeout=120s
kubectl logs job/clusdr-sidecar-bootstrap
```

Copy the `join token :` line if you will join by hand. Delete the Job so the StatefulSet can mount the RWO volume — leaving it running holds the PVC and the STS pod stays `Pending`.

Manifest: [`sidecar-bootstrap.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/sidecar-bootstrap.yaml).

## 2. Start the StatefulSet

```bash
kubectl apply -f examples/k8s/sidecar-statefulset.yaml
kubectl rollout status statefulset/clusdr --timeout=180s
```

Headless Service DNS is the advertised `node.addr` / `raft.addr` (`$(POD_NAME).<headless>.<ns>.svc:7947`). The sidecar binds `0.0.0.0:7947`. The app in that pod dials `127.0.0.1:7947`. A ClusterIP in front of every replica is Consul/etcd and is not `Local()`.

## 3. Join ordinals ≥ 1

Token from step 1. Seed Runtime is ordinal 0’s headless name (see the STS example). Or apply the Sidecar `ClusdrCluster` and let the Operator join so you do not script `join` yourself.

`emptyDir` instead of a PVC forgets `node.id` on restart: crash becomes a new join, not [presence](../concepts/presence.md). Use a PVC per ordinal.

## Checkpoint

Three pods, three `alive` members (`clusdr members` from inside ordinal 0). The app uses `Local()` with no `CLUSDR_GRPC_ADDR`. The Helm chart does not install this topology.

Walkthrough notes: [examples/k8s README](https://github.com/clusdr/clusdr/blob/main/examples/k8s/README.md#sidecar-statefulset-exception).
