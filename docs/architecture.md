# Platform engine architecture

The source is one Go module with three runtime composition roots. A domain module
is a code boundary, not an automatic process or repository boundary.

```text
cmd/search → app.RunSearch → internal/search
                           → platform clients + gRPC lifecycle
cmd/engine → app.RunEngine → platform gRPC health + lifecycle
cmd/worker → app.RunWorker → config + logger + context cancellation
```

Search owns models, repository and cache interfaces, search behavior, gRPC mapping
and Elasticsearch query construction. Redis implements the consumer's cache
interface structurally, without a platform-to-search import. Platform code owns
technical client setup, logger creation, configuration and process lifecycle.

The Search contract and data remain compatible. Pagination defaults to page 1 and
20 items; Elasticsearch filters, publishedAt descending sort and the Redis cache
key format are preserved. There are no data/index migrations in foundation.
Legacy Search environment variables remain fallbacks during the image transition.

Search and engine expose standard gRPC Health. Search's empty service and
`nexusestate.search.v1.SearchService` readiness statuses follow an Elasticsearch
probe every five seconds, with a two-second timeout. No Redis check gates health.
Engine is ready after bootstrap because it has no domain dependencies yet.

SIGTERM/SIGINT cancel the root context. Readiness monitoring stops, health reports
NOT_SERVING, and gRPC drains for up to ten seconds before forced Stop. Redis and
Elasticsearch idle connections close and the logger syncs. Allow more than ten
seconds in an eventual Kubernetes termination grace period. Worker waits directly
on cancellation without a dummy polling loop.

Telemetry exporters and worker HTTP health are deferred until the observability
contract and first deployable worker workload exist. No fake business RPCs, event
consumers, NATS configuration, database placeholders or speculative modules are
included. Add feature modules under `internal/<domain>` and compose them explicitly
when their actual use cases and contracts are ready.

CI owns builds and GHCR publication. The infra repository owns immutable image
selection and ArgoCD rollout. Initially roll out Search only; retain old images and
revert the infra image reference if staging or production verification fails.
