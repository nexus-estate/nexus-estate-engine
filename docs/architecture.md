# Nexus Estate Engine architecture

The source is one Go module with three runtime composition roots. A domain module
is a code boundary, not an automatic process or repository boundary.

```text
cmd/search → app.RunSearch → internal/search
                           → platform clients + gRPC lifecycle
cmd/core   → app.RunCore   → platform gRPC health + lifecycle
cmd/worker → app.RunWorker → config + logger + context cancellation
```

Search owns models, repository and cache interfaces, search behavior, gRPC mapping
and Elasticsearch query construction. Redis implements the consumer's cache
interface structurally, without a platform-to-search import. Platform code owns
technical client setup, logger creation, configuration and process lifecycle.

## Marketplace projection model

`internal/marketplace` owns the canonical Listing-centric projection document that
indexing, reindex and Search v2 will materialize. API/PostgreSQL remains the source
of truth; the Engine owns only derived state. A document may be created only for a
listing the API has already confirmed public and searchable, so the projection
carries no lifecycle status field and never re-derives the publication decision
from status. Materializing the document is the public assertion.

Document identity is `listing_id`. `property_id` describes the underlying supply
asset and never substitutes for the listing identity, because one property can be
published more than once. Field names are snake_case and are the canonical
index/wire contract: identity and `aggregate_version` plus the searchable text,
range, geo (`location` as the Elasticsearch `geo_point` object form), media
summary and `published_at`/`updated_at` timestamps. The matching Elasticsearch
index is checked in as `marketplace.ListingIndexDefinition`: a `dynamic: strict`
create-index body with one shard, one replica and a case- and diacritic-folding
text analyzer. A unit test fails when the definition and the document contract
drift apart. Integration tests create the index in a real Elasticsearch, prove the
strict and geo contract, and pin a relevance set for Vietnamese compound words,
unaccented place names and mixed Vietnamese/English listings. The set also records
where folding is not enough: tokenization is syllable-level, so `match` is
order-insensitive and a single shared syllable of a multi-syllable place name
over-matches unless an explicit AND operator or `match_phrase` is used. Folding
makes the accented and ASCII spellings of one place name collide by design, while
keyword fields stay exact per source value, so the write layer must send the source
value for term filters. The same set covers the numeric and geo fields: price and
area range bounds are asserted to be inclusive, and geo_distance is asserted to
exclude a listing whose location is absent, including when filters compose.
Ordering keeps the recency contract v1 applies to its `publishedAt` sort, including
for filtered and paged results, and extends it to the tuple (`published_at`,
`listing_id`). v1 sorts by publication time alone, so listings sharing an instant —
and any page boundary that splits them — have no deterministic order there;
`listing_id` is the only field that is unique, required and indexed on every
document, so it is the tiebreaker that closes that gap. The tuple is applied in the
requested direction, through the query rather than `index.sort`, so scoring behavior
is unchanged. The index name, lifecycle policy and write path belong to the indexing
plan.

An incoming document whose `aggregate_version` is lower than or equal to the
already indexed version is stale and must become a no-op; only a greater version
supersedes the indexed document. `ValidateMarketplaceListingDocument` rejects an
invalid document before any Elasticsearch write and reports a typed, inspectable
permanent input error. The helpers are pure, hold no state and are safe for
concurrent use.

`internal/search` may consume this model through
`MarketplaceDocumentToPropertySearchItem`, which keeps the item id equal to the
listing id, but Search v1 query construction, cache keys, pagination and the
`nexusestate.search.v1` contract remain unchanged.

The Search contract and data remain compatible. Pagination defaults to page 1 and
20 items; Elasticsearch filters, publishedAt descending sort and the Redis cache
key format are preserved. There are no data/index migrations in foundation.
Legacy Search environment variables remain fallbacks during the image transition.

Search and core expose standard gRPC Health. Search's empty service and
`nexusestate.search.v1.SearchService` readiness statuses follow an Elasticsearch
probe every five seconds, with a two-second timeout. No Redis check gates health.
Core is ready after bootstrap because it has no domain dependencies yet. Core
uses only `CORE_*` configuration and does not initialize Search infrastructure.

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
selection and ArgoCD rollout. Roll out Core, Worker and Search only after their
runtime-specific quality gates pass; retain old Search images and the temporary
legacy Service alias until API consumer migration is verified. Revert the exact
immutable image reference if staging or production verification fails.
