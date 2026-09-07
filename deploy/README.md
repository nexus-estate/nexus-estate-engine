# Deployment boundary and local smoke test

CI publishes `ghcr.io/nexus-estate/nexus-{search,engine,worker}:<full-git-sha>`.
The GitOps repository selects the immutable image and performs rollout. This
repository has no Kubernetes manifests or cluster mutation commands. Keep the
existing Search Service DNS name and gRPC port 50052 during the image transition.
Retain the old image/tag; rollback changes only the infra image reference.
Engine and worker should not be deployed until they have real workloads.

For a disposable local smoke test, use a dedicated Compose project and free ports.
The fixture commands below write only to that local test Elasticsearch index:

```sh
COMPOSE_PROJECT_NAME=nexus-foundation-check ELASTICSEARCH_PORT=19200 REDIS_PORT=16379 make dev-search
curl --fail -X PUT http://localhost:19200/nexus_estate_properties \
  -H 'Content-Type: application/json' \
  -d '{"mappings":{"properties":{"publishedAt":{"type":"date"}}}}'
curl --fail -X PUT 'http://localhost:19200/nexus_estate_properties/_doc/foundation-1?refresh=true' \
  -H 'Content-Type: application/json' \
  -d '{"id":"foundation-1","title":"foundationfixture","publishedAt":"2026-01-01"}'
SEARCH_INTEGRATION_ADDR=localhost:50052 go test -count=1 ./test/integration
# Verify fallback after optional Redis becomes unavailable:
docker compose -p nexus-foundation-check stop redis
SEARCH_INTEGRATION_ADDR=localhost:50052 go test -count=1 ./test/integration
# Stop only the test project's containers; volumes are retained.
docker compose -p nexus-foundation-check down
```

This checks standard gRPC readiness and the original SearchProperties RPC against
real Elasticsearch. The default unit/race suite requires no external services;
shutdown unit tests also exercise a blocked RPC and deadline-based forced stop.
