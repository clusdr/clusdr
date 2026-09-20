<p align="center">
  <a href="https://clusdr.io/docs/guide/kubernetes-operator">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/clusdr/clusdr/main/docs/assets/clusdr-k8s-lettermark-dark.svg">
      <img src="https://raw.githubusercontent.com/clusdr/clusdr/main/docs/assets/clusdr-k8s-lettermark.svg" alt="clusdr operator" width="160" height="164">
    </picture>
  </a>
</p>

<p align="center">Reconciles one <code>ClusdrCluster</code>. Same CLI in-cluster. Not the daemon image.</p>

<p align="center">
  <a href="https://clusdr.io/docs/guide/kubernetes-operator"><img src="https://img.shields.io/badge/docs-clusdr.io-0C0C10" alt="docs"></a>
  <a href="https://github.com/clusdr/clusdr/releases"><img src="https://img.shields.io/github/v/release/clusdr/clusdr?label=release" alt="release"></a>
  <a href="https://hub.docker.com/r/durguto/clusdr-operator"><img src="https://img.shields.io/docker/pulls/durguto/clusdr-operator" alt="image"></a>
  <a href="https://github.com/clusdr/clusdr/blob/main/LICENSE"><img src="https://img.shields.io/github/license/clusdr/clusdr" alt="License"></a>
</p>

---

Kubernetes Operator for [clusdr](https://hub.docker.com/r/durguto/clusdr). It talks to daemons over the Runtime/Control API. It does not embed Raft. It does not inject sidecars. Helm stays DaemonSet.

Crash is `member.dead`. `spec.leave` is `clusdr leave` (`member.left`). A bounced pod is not leave.

## Supported tags

| Tag | What it is |
|---|---|
| `v0.2.1`, `0.2.1` | Current release (same image) |
| `latest` | Same image as the newest `vX.Y.Z` |

Platforms: **linux/amd64**, **linux/arm64**. Distroless, non-root (`65532`). No shell.

[Dockerfile.operator](https://github.com/clusdr/clusdr/blob/main/Dockerfile.operator)

## Quick reference

- **Docs:** [Operator](https://clusdr.io/docs/guide/kubernetes-operator) · [Kubernetes](https://clusdr.io/docs/guide/kubernetes)
- **Daemon image:** [`durguto/clusdr`](https://hub.docker.com/r/durguto/clusdr) (same tag)
- **Source / issues:** [github.com/clusdr/clusdr](https://github.com/clusdr/clusdr)

## How to use this image

Do not `docker run` this. Apply the published YAML; it pins this image.

```bash
kubectl apply -f https://clusdr.io/download/clusdr-operator-bundle.yaml
kubectl apply -f https://raw.githubusercontent.com/clusdr/clusdr/main/examples/k8s/clusdrcluster.yaml
```

Pin with `clusdr-operator-bundle-0.2.1.yaml`, or the two-file apply (`clusdr-crds.yaml` then `clusdr-operator.yaml`). Sidecar CR: `examples/k8s/clusdrcluster-sidecar.yaml`. Do not apply DaemonSet and Sidecar CRs unless you mean two clusters.

## Image contract

| | |
|---|---|
| `ENTRYPOINT` | `/clusdr-operator` |
| User | `65532:65532` (distroless nonroot) |
| Daemon image | `durguto/clusdr` (set on the CR, not this image) |

This image is not `durguto/clusdr`. Members still run the daemon.
