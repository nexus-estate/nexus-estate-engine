# Platform runtime contract

The source repository is the `nexus-estate-platform-engine` monorepo. Runtime
artifacts have independent names and can be built and published independently.

| Runtime | Binary | Image | Port | Deployment status |
| --- | --- | --- | --- | --- |
| Search | `nexus-search` | `ghcr.io/nexus-estate/nexus-search:<full-sha>` | gRPC `50052` | Deploy now |
| Engine | `nexus-engine` | `ghcr.io/nexus-estate/nexus-engine:<full-sha>` | gRPC `50051` | Deploy foundation now |
| Worker | `nexus-worker` | `ghcr.io/nexus-estate/nexus-worker:<full-sha>` | None by default | Deploy foundation now |

Search exposes the unchanged protobuf contract:

```text
package: nexusestate.search.v1
service: SearchService
rpc: SearchProperties
```

Search requires Elasticsearch for readiness and may use Redis as an optional
cache. Redis connection failure must not prevent Search results from being
served through Elasticsearch. The platform-engine CI integration gate proves
both the normal Redis path and the Redis-down fallback before a Search image is
published from a push build.

Engine exposes standard `grpc.health.v1.Health` only at foundation stage.
Worker starts, remains idle without a busy loop, handles signals, and exits
within a bounded shutdown timeout; it has no public Service by default.

The infra repository owns immutable image selection and ArgoCD rollout. This
repository never applies Kubernetes manifests or invokes ArgoCD. Keep the
legacy Search Service alias only during consumer migration; it is not a
canonical runtime name.
