# Nexus Estate Engine

Go modular monorepo for Nexus Estate: one `go.mod`, bounded modules under
`internal/`, and a small number of independently deployable binaries. Keeping
modules together lets us evolve shared platform bootstrap and contracts atomically
without creating a repository or microservice for every domain.

| Binary | Entry point | Port | Status |
| --- | --- | --- | --- |
| nexus-search | `cmd/search` | gRPC 50052 | Existing property search; deployment priority |
| nexus-core | `cmd/core` | gRPC 50051 | Health-only synchronous foundation; deployable now |
| nexus-worker | `cmd/worker` | None | Signal-aware foundation; deployable now |

## Run Search locally

Install Docker with Compose, then run:

```sh
make dev-search
docker compose logs -f search
make down
```

This starts Search with Air, Elasticsearch and Redis through the local-only
`docker-compose.dev.yml` override. It does not create an index or migrate existing
data. Supply the existing property index for useful searches; a missing index
returns the existing Elasticsearch error behavior. For an isolated fixture, see
[the smoke test instructions](deploy/README.md). CI uses the base Compose file and
the production-like Search binary without Air.

Default host ports are 50052, 9200 and 6379. If a port is already used, override
`SEARCH_GRPC_PORT`, `ELASTICSEARCH_PORT` or `REDIS_PORT` when invoking Make.
Compose always points Search at container dependency addresses. Core and worker
are available with `docker compose --profile platform up -d --build`.

For host development, use Go 1.25 (minimum version in `go.mod`):

```sh
cp .env.example .env
docker compose up -d elasticsearch redis
make run-search
# Separate terminals, when needed:
make run-core
make run-worker
```

Environment variables override `.env`. Search-specific settings take precedence
over legacy `APP_NAME`, `GRPC_PORT` and `GRPC_REFLECTION_ENABLED` aliases, which
remain supported for existing Search deployment configurations. Legacy aliases do not
configure Core or Worker. Core uses `CORE_SERVICE_NAME`, `CORE_GRPC_PORT` and
`CORE_GRPC_REFLECTION_ENABLED`; reflection defaults off when `APP_ENV=production`;
explicit settings are respected. Worker uses `WORKER_SERVICE_NAME` only and does
not open a network listener. Redis is optional: failure to connect at startup
runs without caching; later read/write errors fall back to Elasticsearch. Each
cache operation has a 500 ms budget, with retries disabled, so an unavailable
cache cannot consume the full Search request deadline.

Search pagination uses page 1 for non-positive pages, limit 20 for non-positive
limits, and caps limits at 100. The response reports the effective page and limit.
Offset pagination remains in use; deep pages remain subject to Elasticsearch
result-window limits.

## Build and verify

```sh
make build              # bin/nexus-search, bin/nexus-core, bin/nexus-worker
make fmt
make vet
make test
make test-race
make lint               # requires golangci-lint v2.13.2
make check              # formatting, vet, lint, race, module tidiness
```

Install the local pre-commit hook once:

```sh
make install-hooks
```

It checks formatting, `go vet`, available `golangci-lint`, all Go tests, and
builds the Search, Core, and Worker binaries. Docker publishing, dependency
integration, and runtime smoke tests remain CI checks. Run it manually with:

```sh
make pre-commit
```

CI uses Go from `go.mod`. Make forwards that version to Docker/Compose; update
`go.mod` and the Dockerfile/Compose direct-build defaults together when upgrading.
Do not keep a stale `GO_VERSION` override in `.env`.

Unit tests cover query/filter construction, pagination, deterministic cache keys,
cache hit/miss/error behavior, gRPC field mapping, config compatibility, dependency
health and forced shutdown. Required CI runs unit/race checks, binary builds, the
real Search integration gate and all three Docker builds. The integration gate is
isolated and deterministic; it seeds Search fixtures, verifies the Search RPC with
Redis, repeats it after Redis is stopped, and checks dependency-driven readiness
without restarting the process. Local smoke tests remain opt-in via
`SEARCH_INTEGRATION_ADDR`, as documented in [deploy/README.md](deploy/README.md).

## Protobuf

The contract currently lives in `proto/search/v1/search.proto`. Generated Go is
checked into `gen/search/v1`; no external contract repository is assumed.
Install the official `protoc` **36.0** release (reported as 7.36.0 in generated
Go headers) and these pinned plugins:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
export PATH="$(go env GOPATH)/bin:$PATH"
make proto
```

The foundation changes only `go_package`: wire package
`nexusestate.search.v1`, service/RPC names, field numbers and optional presence
remain unchanged. Review contract changes before implementation and regenerate
with `make proto`; do not hand-edit generated files. `make proto` checks all three
tool versions against the pins in `Makefile`. CI installs those exact versions,
regenerates, and runs `git diff --exit-code -- proto gen` before tests.

## Modules and deployment

`cmd/*` contains process entry points; `internal/app` composes each runtime.
Search models, repository interface, Elasticsearch query adapter and gRPC adapter
live together in `internal/search`. Technical config, logging, client setup,
gRPC lifecycle and shutdown live in `internal/platform`.

Add a bounded module only when a concrete feature needs it. Keep domain behavior
inside that module and wire it in an app composition root. Do not add empty future
modules, generic managers, cross-module infrastructure imports or a `utils` bucket.
See [architecture details](docs/architecture.md).

```sh
make docker-search
make docker-core
make docker-worker
```

Each Docker target runs as non-root with CA certificates and timezone data.
Search remains the default Docker target. CI builds all three images on PRs
without pushing. The `integration-search` job starts Elasticsearch, Redis and
the real Search runtime, seeds a deterministic fixture, exercises
`SearchProperties`, stops Redis, exercises the Elasticsearch fallback, and
checks that Elasticsearch failure changes readiness without restarting Search.
The job fails if the integration test is skipped. Docker build and publication
wait for quality, binary builds and this integration gate. Successful
main/develop/tag pushes publish:

- `ghcr.io/nexus-estate/nexus-search:<full-git-sha>`
- `ghcr.io/nexus-estate/nexus-core:<full-git-sha>`
- `ghcr.io/nexus-estate/nexus-worker:<full-git-sha>`

Convenience aliases are `develop-latest`, `latest` on main, and the `v*` tag name.
GitOps must pin immutable SHA/version tags. This repository never applies cluster
manifests or invokes ArgoCD. Keep previous Search images available; the infra
repository changes only the Search image reference after staging verification.
