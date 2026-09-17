# Sidecar

A sidecar next to the app is only when that **replica is the member** (a small elected StatefulSet). Shared netns → `127.0.0.1` / `Local()`. That is the one Kubernetes layout where the pod *is* the host for the SDK.

Default remains [one daemon per node](kubernetes.md#default-one-daemon-per-node). Do not apply this next to the DaemonSet example unless you mean **two** clusters. Do not put a clusdr sidecar on every microservice pod. That is not a mesh sidecar.

```text
Pod
├── Application     →  Local()  →  127.0.0.1:7947
└── clusdr sidecar  →  Raft to the other replicas
```

N replicas = N Raft members. Scaling the StatefulSet scales Raft. A Deployment with a clusdr sidecar is the same mistake: replica count is membership.

## Disk and bootstrap

`emptyDir` forgets `node.id` (crash becomes a new join, not a [presence](../concepts/presence.md) restart). Use a PVC per ordinal.

The published image has **no shell**. One pod template cannot mix seed `start --bootstrap` and joiner `start`. Do not `clusdr init` on every ordinal (each init is a new `cluster.id`).

Bootstrap ordinal 0 with a Job on a pre-created PVC, delete that Job (RWO), then start the StatefulSet without `--bootstrap`. Join `-1` and `-2` the same way as on a VM. The [Operator](kubernetes-operator.md) does that sequence when `spec.topology` is `Sidecar`.

Headless Service DNS is the advertised `node.addr` / `raft.addr` (`$(POD_NAME).svc:7947`). The sidecar binds `0.0.0.0:7947`; the app still dials `127.0.0.1:7947`. Do not put a ClusterIP in front and `Dial` it from every replica. The Operator may Dial that headless name to `join`; that is not the app path.

## Manifests

- [`sidecar-bootstrap.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/sidecar-bootstrap.yaml)
- [`sidecar-statefulset.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/sidecar-statefulset.yaml)
- Operator sample: [`clusdrcluster-sidecar.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/clusdrcluster-sidecar.yaml)

Walkthrough: the [examples/k8s README](https://github.com/clusdr/clusdr/blob/main/examples/k8s/README.md#sidecar-statefulset-exception). The [Helm](kubernetes-helm.md) chart does not install this topology.
