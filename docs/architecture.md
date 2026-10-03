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
asset. Listing and Estate are distinct domain identities. The marketplace
document represents a publication, so its identity is `listing_id` regardless
of current database cardinality constraints.

The source field semantics are:

- `listing_id` comes from API `Listing.id`; `property_id` comes from `Estate.id`.
- Title, description, type, purpose, price, area, address, province/ward IDs and
  names, and coordinates come from the API Estate snapshot. Type and purpose keep
  their exact API enum values: `APARTMENT`, `HOUSE`, `VILLA`, `TOWNHOUSE`, `LAND`,
  `OFFICE`, `SHOPHOUSE`, `WAREHOUSE`, `COMMERCIAL`, `HOTEL`, `RESORT`, `FARM`,
  `OTHER`; and `SALE`, `RENT`, `SALE_OR_RENT`. Engine does not lowercase or rename
  them. Price is required PostgreSQL `bigint`, represented as Go `int64` and
  Elasticsearch `long`; zero is valid. The producer must reject an absent source
  price before decoding it into the zero-valued Go field. Area is nullable API
  `decimal(10,2)`, represented as `*float64` and Elasticsearch `double`; nil means
  unknown and is omitted, while a present value must be positive. Description is
  nullable and optional.
- Although the database column is `bigint`, the current API entity/DTO uses
  TypeScript `number`, and listing response mapping calls `Number(estate.price)`.
  JavaScript numbers do not preserve every int64 above 2^53. The future producer
  contract must transmit price losslessly (for example, a decimal integer string
  parsed by Engine) or the API must explicitly constrain values to safe integers;
  an Engine `int64` cannot recover precision already lost upstream.
- Location is `province_id`, `province_name`, `ward_id`, `ward_name`, `address`
  and optional `location`. Province and ward IDs/names are the API source fields.
  There is no authoritative district field; Engine must not infer one from
  address text or a location hierarchy. The canonical projection does not call a
  province a city.
- `published_at` is API `Listing.publishedAt`, set by its publish transition.
  `updated_at` is API `Listing.updatedAt`, carried for audit and never used for
  ordering. Both must come from the source.
- The API has no authoritative slug, district, or cover-image selection, so none
  is present in the canonical document. API `tbl_media` owns image/video URLs and
  `sort_order`. A producer selects only `type == "image"`, orders by
  `sort_order` ascending then media `id` ascending as a stable tie-breaker, and
  truncates to `MaxMarketplaceImages` (20). This bounds the projection and search
  result only; it does not limit API media ownership. The Engine validates the
  bound but does not invent an ordering or cover choice. Description, area, media
  and geocoordinates may be absent.

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
`price` maps to `long`; `area` maps to `double`; province and ward IDs map to
`keyword`; province and ward names and address map to analyzed `text` with exact
`.keyword` subfields; title/address exact-value limits cover the API's full
500-character source range; and optional coordinates map to `geo_point`. `projection_state`
is a strict object for future write metadata. Searchable text uses a case- and
diacritic-folding analyzer. Shard and replica counts belong to environment or
index-lifecycle configuration, not this field contract. Unit and real
Elasticsearch tests cover mapping drift, strictness, exact integer serialization,
filters, geo distance, Vietnamese and mixed-language relevance, deterministic
`published_at`/`listing_id` ordering and pagination. The analyzer uses standard
syllable tokenization: `match` is order-insensitive and common syllables can
over-match unless the query uses AND or `match_phrase`. Folding intentionally
collides accented and ASCII spellings; keyword subfields stay exact and filters
must use the source value.

### Future durable write ordering

The mapping reserves `projection_state.source_revision`,
`projection_state.deleted` and `projection_state.payload_hash` as internal write
metadata. They are deliberately not fields on `MarketplaceListingDocument`.
Their mapping types are `long`, `boolean` and `keyword`, respectively.
`DecideProjectionMutation` is only a pure contract helper; it does not establish
that a producer revision exists or that Elasticsearch writes are already guarded.

The selected future write design is an atomic scripted update of the Elasticsearch
document whose `_id` is `listing_id`. The script compares the incoming source
revision with `projection_state.source_revision` and writes the full canonical
document plus metadata only for a greater revision. Lower revisions are stale;
higher revisions apply. Equal revision, lifecycle and payload hash is an
idempotent no-op. Equal revision with a different lifecycle or payload hash is a
producer conflict that must be surfaced. `payload_hash` is intended to be a
SHA-256 of deterministic canonical mutation content, including its lifecycle,
computed from a fixed canonical representation; unordered maps, runtime-local
metadata and Elasticsearch response metadata are excluded. Hash calculation is
not implemented in this foundation. Every write must use the same atomic guard,
including retries and reindex writes.

Archive/delete writes a durable tombstone at the same listing `_id`, retaining
the greatest source revision, `deleted: true` and its payload hash. Marketplace
queries must filter `projection_state.deleted: false`; tombstones are never
search results. They must not be physically deleted while older events or
reindex snapshots could still arrive. Thus an archive at revision 12 prevents a
delayed upsert at revision 11 from recreating the listing, even after the
searchable document body is removed. Restore is a higher-revision upsert
authorized by an explicit API lifecycle event; the current API does not yet
implement restore.

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
is only a lossy response-shape compatibility primitive; it maps province name to
legacy City and ward name to Ward, leaves District and Slug empty, maps absent
area to zero, and converts the canonical `int64` price to the legacy float field.
No production Search v1 read path calls it. Search v1 must not be pointed at the
marketplace index by changing only its index name: the field names and query
builder still differ.

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
