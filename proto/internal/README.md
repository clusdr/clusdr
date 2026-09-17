# clusdr internal

Daemon-to-daemon gRPC: join, promote, and heartbeat. Not part of the application SDKs.

- BSR: [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal)
- Package: `clusdr.v1alpha1` (imports membership types from [`buf.build/clusdr/api`](https://buf.build/clusdr/api))

Applications should depend on [`buf.build/clusdr/api`](https://buf.build/clusdr/api) only.
