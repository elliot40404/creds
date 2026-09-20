package app

import (
	"slices"
	"testing"
)

func joinedPair(t *testing.T) (a, b *Service, fa, fb *fakePrompter) {
	t.Helper()
	a, fa, _, _ = initVault(t)
	withRemote(t, a)
	if _, err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	st, err := a.SyncStatus()
	if err != nil {
		t.Fatal(err)
	}
	b, fb, _ = newService(t)
	fb.passwords = []string{mainWord}
	if err := b.Join(st.Remote); err != nil {
		t.Fatal(err)
	}
	return a, b, fa, fb
}

func warned(s *Service) bool {
	return slices.Contains(s.Warnings(), passwordChanged)
}

func TestPasswordChangeFromSyncWarns(t *testing.T) {
	t.Parallel()
	a, b, fa, fb := joinedPair(t)
	fa.passwords = []string{mainWord, nextWord, nextWord}
	if err := a.Passwd(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	if warned(a) {
		t.Fatal("local passwd warned")
	}
	if _, err := b.Sync(); err != nil {
		t.Fatal(err)
	}
	b.Warnings()
	if err := b.Unlock(); err != nil || !warned(b) {
		t.Fatalf("no warning after sync: %v", err)
	}
	if err := unlockWith(t, b, fb, nextWord); err != nil {
		t.Fatal(err)
	}
	b.Warnings()
	if err := b.Unlock(); err != nil || warned(b) {
		t.Fatalf("warning kept after password unlock: %v", err)
	}
}
