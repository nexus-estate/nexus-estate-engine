package marketplace

// ProjectionState is write-layer metadata, not part of
// MarketplaceListingDocument. SourceRevision must be supplied by the API's
// future event contract; the Engine must never generate it. Deleted records the
// source-owned lifecycle decision and lets an archive/delete retain ordering
// information after the searchable document is removed from results.
//
// This type and DecideProjectionMutation define pure ordering semantics only.
// The current API does not yet provide SourceRevision or a durable event stream,
// so they do not make the existing Search path or a future indexer safe to run.
type ProjectionState struct {
	SourceRevision int64 `json:"source_revision"`
	Deleted        bool  `json:"deleted"`
}

// MutationDecision is the result a future atomic projection write must honor.
type MutationDecision uint8

const (
	// MutationApply accepts a first state or one with a higher source revision.
	MutationApply MutationDecision = iota
	// MutationNoop is an idempotent replay of the same revision and lifecycle state.
	MutationNoop
	// MutationRejectStale rejects an older event, including an upsert after a newer tombstone.
	MutationRejectStale
	// MutationRevisionConflict identifies a producer violation: one revision carries conflicting lifecycle state.
	MutationRevisionConflict
	// MutationInvalid rejects a non-positive revision, which is outside the future source contract.
	MutationInvalid
)

// DecideProjectionMutation compares an incoming source-owned revision with the
// durable state for one listing. A nil current state means no state has been
// recorded. The write layer must perform this comparison atomically with storing
// an accepted document or tombstone; a read-then-write check is not sufficient.
func DecideProjectionMutation(incoming ProjectionState, current *ProjectionState) MutationDecision {
	if incoming.SourceRevision <= 0 {
		return MutationInvalid
	}
	if current == nil {
		return MutationApply
	}
	if current.SourceRevision <= 0 {
		return MutationInvalid
	}
	switch {
	case incoming.SourceRevision < current.SourceRevision:
		return MutationRejectStale
	case incoming.SourceRevision > current.SourceRevision:
		return MutationApply
	case incoming.Deleted != current.Deleted:
		return MutationRevisionConflict
	default:
		return MutationNoop
	}
}
