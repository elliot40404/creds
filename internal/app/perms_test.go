package app

import "testing"

func TestWarningsCleanVault(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if w := s.Warnings(); len(w) != 0 {
		t.Fatalf("warnings %q", w)
	}
}
