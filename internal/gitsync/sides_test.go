package gitsync

import (
	"context"
	"errors"
	"testing"
)

func TestConflictSides(t *testing.T) {
	t.Parallel()
	_, s := conflictedPair(t)
	ours, theirs, err := s.ConflictSides(context.Background(), "e")
	if err != nil || ours == nil || theirs == nil {
		t.Fatalf("sides %v %v %v", ours, theirs, err)
	}
	if ours.Fields[0].Value != "b" || theirs.Fields[0].Value != "a" {
		t.Fatalf("ours %+v theirs %+v", ours.Fields, theirs.Fields)
	}
	if _, _, err := s.ConflictSides(context.Background(), "missing"); !errors.Is(err, ErrNoConflict) {
		t.Fatalf("missing: %v", err)
	}
}
