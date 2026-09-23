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
			name:     "first state is accepted",
			incoming: ProjectionState{SourceRevision: 1},
			want:     MutationApply,
		},
		{
			name:     "older revision is stale",
			incoming: ProjectionState{SourceRevision: 9},
			current:  &ProjectionState{SourceRevision: 10},
			want:     MutationRejectStale,
		},
		{
			name:     "equal revision is idempotent",
			incoming: ProjectionState{SourceRevision: 10},
			current:  &ProjectionState{SourceRevision: 10},
			want:     MutationNoop,
		},
		{
			name:     "newer revision supersedes",
			incoming: ProjectionState{SourceRevision: 11},
			current:  &ProjectionState{SourceRevision: 10},
			want:     MutationApply,
		},
		{
			name:     "same revision cannot change lifecycle state",
			incoming: ProjectionState{SourceRevision: 10, Deleted: true},
			current:  &ProjectionState{SourceRevision: 10},
			want:     MutationRevisionConflict,
		},
		{
			name:     "zero source revision is invalid",
			incoming: ProjectionState{},
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

func TestArchivedTombstoneRejectsDelayedUpsert(t *testing.T) {
	stored := ProjectionState{SourceRevision: 10}
	archive := ProjectionState{SourceRevision: 12, Deleted: true}
	if got := DecideProjectionMutation(archive, &stored); got != MutationApply {
		t.Fatalf("archive decision = %v, want apply", got)
	}
	stored = archive

	delayedUpsert := ProjectionState{SourceRevision: 11}
	if got := DecideProjectionMutation(delayedUpsert, &stored); got != MutationRejectStale {
		t.Fatalf("delayed upsert decision = %v, want stale rejection", got)
	}
	if stored != archive {
		t.Fatalf("delayed upsert changed tombstone: got %+v, want %+v", stored, archive)
	}
	if got := DecideProjectionMutation(archive, &stored); got != MutationNoop {
		t.Fatalf("archive retry decision = %v, want idempotent no-op", got)
	}
}

func TestLiveMutationAndReindexUseTheSameOrdering(t *testing.T) {
	// A reindex snapshot is an ordinary source revision. If a newer live event has
	// already been applied, replaying the older snapshot cannot overwrite it.
	live := ProjectionState{SourceRevision: 21}
	staleSnapshot := ProjectionState{SourceRevision: 20}
	if got := DecideProjectionMutation(staleSnapshot, &live); got != MutationRejectStale {
		t.Fatalf("stale reindex snapshot decision = %v, want stale rejection", got)
	}

	// Concurrent/retried writes converge on the greatest revision whichever one
	// reaches the atomic write first.
	older := ProjectionState{SourceRevision: 30}
	newer := ProjectionState{SourceRevision: 31, Deleted: true}
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
