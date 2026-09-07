# Deployment boundary and local smoke test

CI publishes `ghcr.io/nexus-estate/nexus-{search,engine,worker}:<full-git-sha>`.
The GitOps repository selects the immutable image and performs rollout. This
repository has no Kubernetes manifests or cluster mutation commands. Keep the
Search gRPC contract and port 50052 stable during the runtime transition. The
infra repository provides the canonical `nexus-search` Service and temporary
legacy aliases. Retain the old image/tag; rollback changes only the infra image
reference or Service selector.
Engine and worker are deployable lifecycle foundations now. They expose no
fake business modules: Engine provides only standard gRPC health, while Worker
starts, stays idle without a busy loop, and shuts down on signal. Their infra
manifests initialize them with one replica in staging and production.

CI runs the same disposable Compose flow as a required `integration-search`
gate. It fails when the integration test is skipped and runs the Search RPC
both before and after Redis is stopped.

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
SEARCH_INTEGRATION_ADDR=localhost:50052 SEARCH_INTEGRATION_KEYWORD=foundationfallback SEARCH_INTEGRATION_EXPECTED_ID=foundation-fallback go test -count=1 -run '^TestSearchRuntime$' ./test/integration
# Elasticsearch outage changes readiness but does not stop the Search process:
docker compose -p nexus-foundation-check stop elasticsearch
SEARCH_INTEGRATION_ADDR=localhost:50052 SEARCH_EXPECT_NOT_SERVING=1 go test -count=1 -run '^TestSearchReadinessDropsWhenElasticsearchStops$' ./test/integration
# Verify fallback after optional Redis becomes unavailable:
docker compose -p nexus-foundation-check stop redis
SEARCH_INTEGRATION_ADDR=localhost:50052 go test -count=1 ./test/integration
# Stop only the test project's containers; volumes are retained.
docker compose -p nexus-foundation-check down
```

This checks standard gRPC readiness and the original SearchProperties RPC against
real Elasticsearch. The default unit/race suite requires no external services;
shutdown unit tests also exercise a blocked RPC and deadline-based forced stop.
