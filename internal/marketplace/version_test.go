package marketplace

import "testing"

func TestDecideProjectionMutation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		incoming ProjectionState
		current  *ProjectionState
		want     MutationDecision
	}{
		{
			name:     "first revision applies",
			incoming: ProjectionState{SourceRevision: 1, PayloadHash: "sha256:one"},
			want:     MutationApply,
		},
		{
			name:     "older revision is stale",
			incoming: ProjectionState{SourceRevision: 9, PayloadHash: "sha256:old"},
			current:  &ProjectionState{SourceRevision: 10, PayloadHash: "sha256:current"},
			want:     MutationRejectStale,
		},
		{
			name:     "equal revision same lifecycle and hash is idempotent",
			incoming: ProjectionState{SourceRevision: 10, PayloadHash: "sha256:same"},
			current:  &ProjectionState{SourceRevision: 10, PayloadHash: "sha256:same"},
			want:     MutationNoop,
		},
		{
			name:     "newer revision applies",
			incoming: ProjectionState{SourceRevision: 11, PayloadHash: "sha256:new"},
			current:  &ProjectionState{SourceRevision: 10, PayloadHash: "sha256:old"},
			want:     MutationApply,
		},
		{
			name:     "same revision different lifecycle conflicts",
			incoming: ProjectionState{SourceRevision: 10, Deleted: true, PayloadHash: "sha256:delete"},
			current:  &ProjectionState{SourceRevision: 10, PayloadHash: "sha256:live"},
			want:     MutationRevisionConflict,
		},
		{
			name:     "same revision different payload conflicts",
			incoming: ProjectionState{SourceRevision: 10, PayloadHash: "sha256:other"},
			current:  &ProjectionState{SourceRevision: 10, PayloadHash: "sha256:current"},
			want:     MutationRevisionConflict,
		},
		{
			name:     "zero source revision is invalid",
			incoming: ProjectionState{PayloadHash: "sha256:payload"},
			want:     MutationInvalid,
		},
		{
			name:     "missing payload hash is invalid",
			incoming: ProjectionState{SourceRevision: 1},
			want:     MutationInvalid,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := DecideProjectionMutation(tc.incoming, tc.current); got != tc.want {
				t.Fatalf("DecideProjectionMutation(%+v, %+v) = %v, want %v", tc.incoming, tc.current, got, tc.want)
			}
		})
	}
}

func TestArchivedTombstoneRejectsDelayedUpsertAndReplaysIdempotently(t *testing.T) {
	stored := ProjectionState{SourceRevision: 10, PayloadHash: "sha256:live-v10"}
	archive := ProjectionState{SourceRevision: 12, Deleted: true, PayloadHash: "sha256:archive-v12"}
	if got := DecideProjectionMutation(archive, &stored); got != MutationApply {
		t.Fatalf("archive decision = %v, want apply", got)
	}
	stored = archive

	delayedUpsert := ProjectionState{SourceRevision: 11, PayloadHash: "sha256:live-v11"}
	if got := DecideProjectionMutation(delayedUpsert, &stored); got != MutationRejectStale {
		t.Fatalf("delayed upsert decision = %v, want stale rejection", got)
	}
	if stored != archive {
		t.Fatalf("delayed upsert changed tombstone: got %+v, want %+v", stored, archive)
	}
	if got := DecideProjectionMutation(archive, &stored); got != MutationNoop {
		t.Fatalf("archive retry decision = %v, want idempotent no-op", got)
	}

	conflictingArchive := archive
	conflictingArchive.PayloadHash = "sha256:conflicting-archive"
	if got := DecideProjectionMutation(conflictingArchive, &stored); got != MutationRevisionConflict {
		t.Fatalf("same revision archive with different hash = %v, want conflict", got)
	}
	conflictingLifecycle := archive
	conflictingLifecycle.Deleted = false
	if got := DecideProjectionMutation(conflictingLifecycle, &stored); got != MutationRevisionConflict {
		t.Fatalf("same revision different lifecycle = %v, want conflict", got)
	}

	restore := ProjectionState{SourceRevision: 13, PayloadHash: "sha256:restore-v13"}
	if got := DecideProjectionMutation(restore, &stored); got != MutationApply {
		t.Fatalf("explicit newer restore decision = %v, want apply", got)
	}
}

func TestLiveMutationAndReindexUseTheSameOrdering(t *testing.T) {
	// A reindex snapshot is an ordinary source revision. If a newer live event has
	// already been applied, replaying the older snapshot cannot overwrite it.
	live := ProjectionState{SourceRevision: 21, PayloadHash: "sha256:live-v21"}
	staleSnapshot := ProjectionState{SourceRevision: 20, PayloadHash: "sha256:snapshot-v20"}
	if got := DecideProjectionMutation(staleSnapshot, &live); got != MutationRejectStale {
		t.Fatalf("stale reindex snapshot decision = %v, want stale rejection", got)
	}

	// Concurrent/retried writes converge on the greatest revision whichever one
	// reaches the atomic write first.
	older := ProjectionState{SourceRevision: 30, PayloadHash: "sha256:live-v30"}
	newer := ProjectionState{SourceRevision: 31, Deleted: true, PayloadHash: "sha256:delete-v31"}
	for _, order := range [][2]ProjectionState{{older, newer}, {newer, older}} {
		var current *ProjectionState
		for _, incoming := range order {
			if DecideProjectionMutation(incoming, current) == MutationApply {
				accepted := incoming
				current = &accepted
			}
		}
		if current == nil || *current != newer {
			t.Fatalf("order %v ended with %v, want newest revision %+v", order, current, newer)
		}
	}
}
