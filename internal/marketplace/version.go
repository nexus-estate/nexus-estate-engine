package marketplace

// Projection write semantics for aggregate versions:
//
//	incoming <= indexed -> stale, the write layer must no-op
//	incoming >  indexed -> the incoming revision supersedes the indexed document
//
// The write layer owns retry and concurrency policy; this file only defines the
// pure comparison so every consumer agrees on what "stale" means.
//
// The helpers hold no state and are safe for concurrent use.

// CompareAggregateVersion returns -1 when incoming is older than indexed, 0 when
// they are equal, and 1 when incoming is newer.
func CompareAggregateVersion(incoming, indexed int64) int {
	switch {
	case incoming > indexed:
		return 1
	case incoming < indexed:
		return -1
	default:
		return 0
	}
}

// IsStaleVersion reports whether an incoming document must be treated as a
// no-op because its revision is older than or equal to the indexed revision.
// Re-projecting the same version therefore never produces a second write.
func IsStaleVersion(incoming, indexed int64) bool {
	return incoming <= indexed
}
