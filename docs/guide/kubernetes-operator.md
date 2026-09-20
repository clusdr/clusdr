# Install the Operator

`clusdr-operator` reconciles one `ClusdrCluster` by talking to daemons over the Runtime API and running the same `init` / `--bootstrap` / `join` sequence a human would. It does not embed Raft. Join is automatic; `leave` is only `spec.leave` — deleting a pod is not leave, so a bounce must not shrink quorum.

Field list: [ClusdrCluster](../reference/clusdrcluster.md). Why DaemonSet vs Sidecar: [Kubernetes](../concepts/kubernetes.md).

## 1. Install the Operator

```bash
kubectl apply -f https://clusdr.io/download/clusdr-operator-bundle.yaml
```

That file is CRDs, then RBAC + Deployment. Pin a tag with `clusdr-operator-bundle-0.2.0.yaml`. The two-file apply (`clusdr-crds.yaml` then `clusdr-operator.yaml`, or the versioned names) stays as pin/fallback. Contributor: `kubectl apply -k config/crd` then `kubectl apply -k config/operator`.

Image: `durguto/clusdr-operator` (GHCR `ghcr.io/clusdr/clusdr-operator`). Same tag as the daemon. The CRD is not in the Helm chart — installing only the chart leaves `kubectl get clusdrcluster` unknown. There is no Helm chart of the Operator.

## 2. Apply a cluster object

The Operator creates a prepare DaemonSet (`mkdir` + `chown 65532` on `dataDir`). Do not `docker exec` onto the node. Omit `spec.seedNodeName`: it picks the first Ready node, writes `status.seedNodeName` on the CR (that write is the lock), and only then creates init + `--bootstrap` pinned to that name. A bounced operator pod reads the stored name — it does not start a second init. Set `spec.seedNodeName` if you need a specific node; that value wins.

```bash
kubectl apply -f https://raw.githubusercontent.com/clusdr/clusdr/main/examples/k8s/clusdrcluster.yaml
```

Sidecar sample: [`clusdrcluster-sidecar.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/clusdrcluster-sidecar.yaml). Sidecar does not auto-pick a node (PVC-0 is the seed). Do not apply both unless you mean **two** clusters (two Raft groups, two tokens). `kubectl get clusdrcluster` then shows WARNING `two clusters` — that is a printer column, not a webhook.

`voterCount` must be odd; the CRD CEL rule rejects even values at apply time.

## 3. Watch it form

```bash
kubectl get clusdrcluster clusdr -n clusdr -w
```

Phase moves `Pending` → `Ready`. Token is a Secret, not git. Distroless has no shell: the Operator runs seed `init` (and sidecar `--bootstrap` on PVC-0, then deletes that Job so the RWO volume can mount).

If phase stays `Pending` with “waiting for sidecar bootstrap” or seed init, the Job failed — describe it. `Error` + “join token missing” means init logs were empty; fix the Job, do not hand-edit Raft.

## 4. Leave (only when you mean it)

A missing or restarted pod is **not** leave. Cordon, drain, eviction, and PreStop are the same reboot story (`member.dead`, then `clusdr start` on the same hostPath). Do not put `leave` on those hooks. To drop a Raft id for good today:

```bash
kubectl get clusdrcluster clusdr -n clusdr -o jsonpath='{.status.members}' ; echo
kubectl patch clusdrcluster clusdr -n clusdr --type merge \
  -p '{"spec":{"leave":["<id-from-status.members>"]}}'
```

Bounce a member (same node, same hostPath) and check the id stays:

```bash
kubectl delete pod -n clusdr -l app.kubernetes.io/component=member
kubectl get clusdrcluster clusdr -n clusdr -o jsonpath='{.status.members}' ; echo
```

If the id disappeared, you patched `leave` or the disk was empty. A later automation (not shipped) may `leave` only when the Kubernetes **Node object is deleted**. Not PreStop.

## 5. Point your app at this node

A pod’s `127.0.0.1` is that pod, not the node daemon. Copy this Deployment. Sidecar topology still uses `Local()` on `127.0.0.1` — do not set `hostIP` there. Not a webhook, not a kube SDK.

```yaml
# Not a clusdr member. Not a Service of clusdr.
# Sidecar topology: skip this — Local() on 127.0.0.1.
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: default
  labels:
    app.kubernetes.io/name: app
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: app
    spec:
      containers:
        - name: app
          image: your.registry/app:tag
          env:
            - name: NODE_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.hostIP
            - name: CLUSDR_GRPC_ADDR
              value: "$(NODE_IP):7947"
            - name: CLUSDR_DATA_DIR
              value: /var/lib/clusdr
            # Dev without PEMs: CLUSDR_TLS=disabled on daemon and app.
            # - name: CLUSDR_TLS
            #   value: disabled
          volumeMounts:
            - name: clusdr-data
              mountPath: /var/lib/clusdr
              readOnly: true
      volumes:
        - name: clusdr-data
          hostPath:
            path: /var/lib/clusdr
            type: Directory
```

Same file: [`examples/k8s/app.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/app.yaml).

## Checkpoint

`kubectl get clusdrcluster` shows Phase `Ready`, a Leader, Seed, and an empty WARNING unless a second CR exists. `status.members` matches `clusdr members` on the seed. Sidecar replicas = `voterCount`; scaling a workload Deployment does not add Raft members.
