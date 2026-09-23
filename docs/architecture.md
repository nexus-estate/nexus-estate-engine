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

## Marketplace projection foundation

`internal/marketplace` defines the canonical Listing-centric projection shape for
future indexing and marketplace search. API/PostgreSQL remains the transactional
source of truth; Engine owns only derived search state. The projection has no
status field. Only an upstream API lifecycle decision may cause a listing to be
materialized or hidden; Engine must not infer public visibility from the presence
of fields in an Elasticsearch document.

`listing_id` is the projection identity, and `property_id` identifies its supply
asset. The API schema currently enforces a unique Estate id on Listing, so there
can be at most one Listing row per Estate. Listing and Estate remain distinct
domain identities, and the publication document is keyed by `listing_id`
regardless of that current cardinality.

The source field semantics are:

- `listing_id` comes from API `Listing.id`; `property_id` comes from `Estate.id`.
- Title, type, purpose, price, area, address, province/city, ward and coordinates
  come from the API Estate snapshot. API requires price and accepts zero as a real
  value. Area is nullable and, when present, must be positive; nil means unknown
  and is omitted from the indexed source so range queries do not treat it as 0.
  Description is nullable and optional.
- `published_at` is API `Listing.publishedAt`, set by its publish transition.
  `updated_at` is API `Listing.updatedAt`, carried for audit and never used for
  ordering. Both must come from the source.
- The current API Listing/Estate contract has no slug or district field. The
  producer may pass those fields only when backed by an authoritative API source;
  Engine must not generate a slug or infer a district. Media lives in API
  `tbl_media` as estate-owned URLs with type and sort order; the producer must
  select image URLs by ascending `sort_order` and apply a documented bound. The
  current API has no cover-image choice, so `cover_image` stays empty unless that
  choice is made authoritative upstream. Missing optional text/media/geo remains
  absent.

The API currently implements `DRAFT → PUBLISHED → ARCHIVED` and stores
`published_at` and `updated_at`, but the inspected API has no transactional
outbox/event envelope, no monotonic per-listing revision, and no Listing restore
operation. `updated_at` is an `@UpdateDateColumn`, not a version. Therefore
indexing cannot be activated until the API publishes an authoritative event or
snapshot containing the lifecycle decision and a persistent, strictly
monotonic-per-listing source revision. The revision must advance for every source
change represented in the projection, including Estate, media and referenced
location changes, plus publication lifecycle changes, and must be committed
atomically with the source mutation and its outbox record. Engine must not derive
it from a timestamp, Kafka offset, random value or local counter. A future restore
must arrive as an explicit upstream lifecycle event.

The canonical document uses snake_case fields and a strict Elasticsearch mapping.
Price is required and may be zero. Area is optional and nullable. Geo is an
optional `geo_point`; media images and cover are bounded source data. Searchable
text uses a case- and diacritic-folding analyzer. Shard and replica counts belong
to environment/index-lifecycle configuration, not this canonical field mapping.
Unit tests check document validation and mapping drift; real Elasticsearch tests
cover strict mapping, round trip, price/area and geo filters, Vietnamese and mixed
language relevance, deterministic `published_at`/`listing_id` ordering and
pagination. The analyzer uses standard syllable tokenization: `match` is
order-insensitive and common syllables can over-match unless the query uses AND
or `match_phrase`. Folding intentionally collides accented and ASCII spellings;
keyword subfields remain exact and filters must use the source value.

### Future durable write ordering

The mapping reserves `projection_state.source_revision` and
`projection_state.deleted` as internal write metadata. They are deliberately not
fields on `MarketplaceListingDocument`. `DecideProjectionMutation` is only a pure
contract helper; it does not establish that a producer revision exists or that
Elasticsearch writes are already guarded.

The selected future write design is an atomic scripted update of the Elasticsearch
document whose `_id` is `listing_id`. The script compares the incoming source
revision with `projection_state.source_revision` and writes the full canonical
document plus metadata only for a greater revision. An equal revision and same
lifecycle state is an idempotent no-op; an equal revision with conflicting
lifecycle state is a producer error; a lower revision is rejected. Every write
must use this same guard, including retry and reindex writes.

Archive/delete writes a durable tombstone at the same listing `_id`, retaining
the greatest source revision and setting `deleted: true`. Marketplace queries
must filter `projection_state.deleted: false`; tombstones are never search
results. They must not be physically deleted while older events or reindex
snapshots could still arrive. Thus an archive at revision 12 prevents a delayed
upsert at revision 11 from recreating the listing. Restore is a higher-revision
upsert authorized by an explicit API lifecycle event.

Reindex reads a source-consistent checkpoint and carries the same revision and
lifecycle state as live events. It seeds archived/deleted state as tombstones,
then replays events after that checkpoint through the same atomic guard before
switching the read alias. Retried and concurrent operations converge on the
highest revision for that listing. Equal-revision payload disagreement is a
source contract violation and must be surfaced rather than silently accepted.

### Search compatibility boundary

Search v1 remains on its legacy `nexus_estate_properties` schema (`id`,
`publishedAt`), query builder, decoder, sort and production index configuration.
The marketplace index uses `listing_id`, `published_at` and a distinct
`projection_state` filter. It needs its own query builder and decoder before it
can serve Search v1 or Search v2 requests. `MarketplaceDocumentToPropertySearchItem`
is only a response-shape compatibility primitive; no production Search v1 read
path calls it. Search v1 must not be pointed at the marketplace index by changing
only its index name.

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
