# Nexus Estate Engine runtime contract

The source repository is the `nexus-estate-engine` Go subsystem. Runtime
artifacts have independent names and can be built and published independently.

| Runtime | Binary | Image | Port | Deployment status |
| --- | --- | --- | --- | --- |
| Search | `nexus-search` | `ghcr.io/nexus-estate/nexus-search:<full-sha>` | gRPC `50052` | Deploy now |
| Core | `nexus-core` | `ghcr.io/nexus-estate/nexus-core:<full-sha>` | gRPC `50051` | Deploy foundation now |
| Worker | `nexus-worker` | `ghcr.io/nexus-estate/nexus-worker:<full-sha>` | None by default | Deploy foundation now |

Search exposes the unchanged protobuf contract:

```text
package: nexusestate.search.v1
service: SearchService
rpc: SearchProperties
```

Search requires Elasticsearch for readiness and may use Redis as an optional
cache. Redis connection failure must not prevent Search results from being
served through Elasticsearch. The Search integration CI gate proves
both the normal Redis path and the Redis-down fallback before a Search image is
published from a push build.

Core exposes standard `grpc.health.v1.Health` only at foundation stage and has
no business RPCs until a real synchronous module is introduced.
Worker starts, remains idle without a busy loop, handles signals, and exits
within a bounded shutdown timeout; it has no public Service by default.

The infra repository owns immutable image selection and ArgoCD rollout. This
repository never applies Kubernetes manifests or invokes ArgoCD. Keep the
legacy Search Service alias only during consumer migration; it is not a
canonical runtime name.

The canonical GitOps source paths are maintained in `nexus-estate-infra`:
`k8s/engine/search`, `k8s/engine/core` and `k8s/engine/worker`, each with
staging and production overlays. This repository defines the runtime contracts
and CI gates but does not contain or apply those manifests.
