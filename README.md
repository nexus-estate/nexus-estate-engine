# Nexus Estate Platform Engine

Go modular monorepo for Nexus Estate: one `go.mod`, bounded modules under
`internal/`, and a small number of independently deployable binaries. Keeping
modules together lets us evolve shared platform bootstrap and contracts atomically
without creating a repository or microservice for every domain.

| Binary | Entry point | Port | Status |
| --- | --- | --- | --- |
| nexus-search | `cmd/search` | gRPC 50052 | Existing property search; deployment priority |
| nexus-engine | `cmd/engine` | gRPC 50051 | Health-only skeleton; deploy later |
| nexus-worker | `cmd/worker` | None | Signal-aware skeleton; deploy when work exists |

## Run Search locally

Install Docker with Compose, then run:

```sh
make dev-search
docker compose logs -f search
make down
```

This starts Search with Air, Elasticsearch and Redis. It does not create an index
or migrate existing data. Supply the existing property index for useful searches;
a missing index returns the existing Elasticsearch error behavior. For an isolated
fixture, see [the smoke test instructions](deploy/README.md).

Default host ports are 50052, 9200 and 6379. If a port is already used, override
`SEARCH_GRPC_PORT`, `ELASTICSEARCH_PORT` or `REDIS_PORT` when invoking Make.
Compose always points Search at container dependency addresses. Engine and worker
are available with `docker compose --profile platform up -d --build`.

For host development, use Go 1.25 (minimum version in `go.mod`):

```sh
cp .env.example .env
docker compose up -d elasticsearch redis
make run-search
# Separate terminals, when needed:
make run-engine
make run-worker
```

Environment variables override `.env`. Search-specific settings take precedence
 over legacy `APP_NAME`, `GRPC_PORT` and `GRPC_REFLECTION_ENABLED` aliases, which
remain supported for existing deployment configurations. Legacy aliases do not
configure engine or worker. Reflection defaults off when `APP_ENV=production`;
explicit settings are respected. Redis is optional: failure to connect at startup
runs without caching; later read/write errors fall back to Elasticsearch.

## Build and verify

```sh
make build              # bin/nexus-search, bin/nexus-engine, bin/nexus-worker
make fmt
make vet
make test
make test-race
make lint               # requires golangci-lint v2
make check              # formatting, vet, lint, race, module tidiness
```

CI uses Go from `go.mod`. Make forwards that version to Docker/Compose; update
`go.mod` and the Dockerfile/Compose direct-build defaults together when upgrading.
Do not keep a stale `GO_VERSION` override in `.env`.

Unit tests cover query/filter construction, pagination, deterministic cache keys,
cache hit/miss/error behavior, gRPC field mapping, config compatibility, dependency
health and forced shutdown. The optional live integration test is documented in
[deploy/README.md](deploy/README.md).

## Protobuf

The contract currently lives in `proto/search/v1/search.proto`. Generated Go is
checked into `gen/search/v1`; no external contract repository is assumed.
Install `protoc` (generated files currently use 7.36.0) and these plugins:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
export PATH="$(go env GOPATH)/bin:$PATH"
make proto
```

The foundation changes only `go_package`: wire package
`nexusestate.search.v1`, service/RPC names, field numbers and optional presence
remain unchanged. Review contract changes before implementation and regenerate
with `make proto`; do not hand-edit generated files.

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
make docker-engine
make docker-worker
```

Each Docker target runs as non-root with CA certificates and timezone data.
Search remains the default Docker target. CI builds all three images on PRs
without pushing; successful main/develop/tag pushes publish:

- `ghcr.io/nexus-estate/nexus-search:<full-git-sha>`
- `ghcr.io/nexus-estate/nexus-engine:<full-git-sha>`
- `ghcr.io/nexus-estate/nexus-worker:<full-git-sha>`

Convenience aliases are `develop-latest`, `latest` on main, and the `v*` tag name.
GitOps must pin immutable SHA/version tags. This repository never applies cluster
manifests or invokes ArgoCD. Keep previous Search images available; the infra
repository changes only the Search image reference after staging verification.
