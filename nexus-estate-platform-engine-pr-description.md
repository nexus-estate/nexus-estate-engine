# Refactor: Bootstrap Modular Platform Engine and Restore CI

## Summary

This PR converts the former standalone Go Search codebase into the initial **Nexus Estate Go modular monorepo**:

```text
nexus-estate-platform-engine
```

The repository now contains three initial deployable runtimes:

```text
nexus-search
nexus-engine
nexus-worker
```

This PR is intentionally limited to platform foundation, Search migration, reliability, build tooling, and CI/CD structure. It does not introduce future business modules such as Pricing, Billing, Subscription, Scheduler, or Notification.

---

## Why

Nexus Estate is moving toward:

```text
NestJS API Platform
+
Go modular platform engine
```

With the current team size, creating a repository and deployment pipeline for every Go domain would add unnecessary operational overhead.

The architecture chosen for the Go side is:

> **Start modular, deploy coarse-grained, split only under pressure.**

That means:

```text
1 Go module
many bounded modules
few independently deployable binaries
```

---

## Architecture

```text
nexus-estate-platform-engine/
├── cmd/
│   ├── search/
│   ├── engine/
│   └── worker/
│
├── internal/
│   ├── app/
│   ├── search/
│   └── platform/
│
├── proto/
├── gen/
├── deploy/
└── test/
```

### `nexus-search`

Current functional runtime.

Responsibilities:

- Search gRPC API.
- Elasticsearch queries.
- optional Redis cache.
- Search readiness.
- Search runtime composition.

Port:

```text
50052
```

### `nexus-engine`

Foundation runtime for future synchronous Go business engines.

Future examples:

```text
pricing
entitlement
ranking
risk
```

Port:

```text
50051
```

No speculative business logic is added in this PR.

### `nexus-worker`

Foundation runtime for future asynchronous workloads.

Future examples:

```text
scheduler
reminder
notification
analytics
event consumers
```

No dummy jobs or speculative queues are added in this PR.

---

## Main changes

### Go module rename

The module has been migrated to:

```text
github.com/nexus-estate/nexus-estate-platform-engine
```

Go imports, protobuf-generated package paths, build scripts, and documentation were updated accordingly.

---

### Search runtime migration

The Search entrypoint moved from:

```text
cmd/server
```

to:

```text
cmd/search
```

Search-specific business logic remains under:

```text
internal/search
```

Technical/runtime concerns are now under:

```text
internal/platform
```

---

### Runtime composition

Added composition roots under:

```text
internal/app
```

for:

```text
search
engine
worker
```

The `cmd/*` entrypoints remain intentionally thin.

---

### Shared platform infrastructure

Introduced technical platform packages for:

- configuration.
- structured logging.
- gRPC lifecycle.
- graceful shutdown.
- Elasticsearch.
- Redis.

Business logic stays outside `internal/platform`.

---

### Graceful shutdown

Search and Engine now support bounded graceful termination:

```text
SIGINT / SIGTERM
→ health NOT_SERVING
→ gRPC GracefulStop
→ force Stop after timeout
```

This prepares the runtimes for Kubernetes rolling updates.

---

### gRPC health

The runtimes use the standard:

```text
grpc.health.v1.Health
```

Search readiness depends on Elasticsearch.

Redis remains an optional cache and does not determine Search readiness.

---

### Redis behavior

Search preserves degraded-mode behavior:

```text
Redis unavailable
→ Search continues using Elasticsearch
```

Redis operations are bounded so cache failures cannot consume the full Search request deadline.

---

### Search pagination safety

Search offset pagination now applies a maximum page size.

Behavior:

```text
page <= 0
→ page 1

limit <= 0
→ default limit

limit above maximum
→ clamp to safe maximum
```

This protects Elasticsearch from excessively large result windows without introducing breaking RPC behavior.

Deep pagination with `search_after` is intentionally deferred to a future Search PR.

---

### Protobuf compatibility

The Search wire contract remains compatible:

```text
package nexusestate.search.v1
SearchService
SearchProperties
port 50052
```

Existing protobuf field numbers and RPC names are unchanged.

CI also verifies that checked-in generated Go files remain synchronized with the `.proto` source.

---

### Docker

One repository now builds three independent runtime images:

```text
ghcr.io/nexus-estate/nexus-search:<tag>
ghcr.io/nexus-estate/nexus-engine:<tag>
ghcr.io/nexus-estate/nexus-worker:<tag>
```

This allows independent deployment and scaling later without splitting the source repository.

---

### CI

CI validates:

```text
gofmt
go vet
golangci-lint
go test -race
go mod tidy
protobuf generation consistency
binary builds
Docker builds
```

All three binaries are compiled.

PR builds do not push images.

Pushes to `develop`, `main`, and release tags can publish GHCR images.

Immutable Git SHA tags are used for GitOps-safe deployment.

Workflow concurrency prevents older runs from racing newer pushes on the same ref.

---

## Backward compatibility

This PR intentionally preserves Search behavior needed for the current deployment.

Preserved:

```text
Search gRPC package
Search RPC names
protobuf field numbers
port 50052
existing Elasticsearch query behavior
Redis optional fallback
```

The Kubernetes Service/DNS name does not need to change as part of the repository migration.

The initial GitOps migration can therefore update only the container image.

---

## Deployment boundary

This repository:

```text
builds and publishes images
```

It does not:

```text
kubectl apply
argocd app sync
modify cluster resources
```

Deployment remains controlled by:

```text
nexus-estate-infra
+
ArgoCD
```

Recommended rollout:

```text
platform-engine CI
↓
publish nexus-search:<git-sha>
↓
update staging image in infra repo
↓
ArgoCD staging rollout
↓
smoke test
↓
production promotion
```

Existing Search Kubernetes Service naming and port can remain unchanged during the first migration.

---

## Tests

Coverage includes:

- configuration.
- gRPC lifecycle.
- graceful shutdown.
- Redis adapter behavior.
- Search service behavior.
- Elasticsearch query construction.
- pagination normalization.
- gRPC Search mapping.
- optional real-stack Search integration.

The real Search smoke/integration test remains opt-in through:

```text
SEARCH_INTEGRATION_ADDR
```

See:

```text
deploy/README.md
```

---

## Local development

Start the Search stack:

```bash
make dev-search
```

Run all local quality checks:

```bash
make check
```

Build all runtimes:

```bash
make build
```

Build individual Docker images:

```bash
make docker-search
make docker-engine
make docker-worker
```

---

## Out of scope

This PR deliberately does not add:

- Pricing.
- Billing.
- Metering.
- Subscription.
- Entitlement business logic.
- Promotion.
- Scheduler implementation.
- Reminder.
- Notification.
- NATS consumers.
- Kafka.
- Temporal.
- Kubernetes manifests.
- ArgoCD application changes.

These will be added only when product features require them.

---

## Follow-up

Recommended next PR:

```text
feat(search): add index lifecycle and indexing boundary
```

Planned scope:

- versioned Elasticsearch indices.
- read/write aliases.
- index upsert/delete boundary.
- reindex tooling.
- backfill support.
- event-ready Search projection.

After that:

```text
NestJS transactional outbox
→ event broker
→ Search index consumer
```

---

## Checklist

### Architecture

- [x] One Go module.
- [x] `cmd/search`.
- [x] `cmd/engine`.
- [x] `cmd/worker`.
- [x] Search bounded module.
- [x] shared technical platform packages.
- [x] thin runtime entrypoints.

### Compatibility

- [x] Search gRPC package unchanged.
- [x] Search RPC names unchanged.
- [x] protobuf field numbers unchanged.
- [x] Search port 50052 unchanged.
- [x] Redis remains optional.

### Reliability

- [x] graceful shutdown.
- [x] standard gRPC health.
- [x] Elasticsearch readiness.
- [x] bounded Redis latency.
- [x] bounded Search page size.

### CI

- [x] gofmt.
- [x] go vet.
- [x] golangci-lint.
- [x] race tests.
- [x] module tidiness.
- [x] protobuf generated-code consistency.
- [x] all binaries build.
- [x] all Docker targets build.
- [x] workflow concurrency protection.

### Deployment

- [x] independent runtime images.
- [x] immutable SHA image tags.
- [x] no direct Kubernetes mutation.
- [x] GitOps deployment boundary documented.

---

## Merge strategy

Recommended:

```text
Squash and merge
```

Suggested squash commit:

```text
refactor: bootstrap modular platform engine
```
