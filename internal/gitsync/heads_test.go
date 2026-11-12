package gitsync

import (
	"strings"
	"testing"
)

func TestParseHeads(t *testing.T) {
	t.Parallel()
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	tests := []struct {
		name, out     string
		local, remote string
		ok            bool
	}{
		{"none", "", "", "", true},
		{"both", "commit " + a + " " + localRef + "\ncommit " + b + " " + remoteRef + "\n", a, b, true},
		{"local only", "commit " + a + " " + localRef + "\n", a, "", true},
		{"tag", "tag " + a + " " + localRef + "\n", "", "", false},
		{"other ref", "commit " + a + " " + localRef + "/x\n", "", "", false},
		{"short id", "commit abc " + localRef + "\n", "", "", false},
		{"duplicate", "commit " + a + " " + localRef + "\ncommit " + b + " " + localRef + "\n", "", "", false},
		{"extra field", "commit " + a + " " + localRef + " x\n", "", "", false},
	}
	for _, tc := range tests {
		local, remote, ok := parseHeads(tc.out)
		if local != tc.local || remote != tc.remote || ok != tc.ok {
			t.Fatalf("%s: got %q %q %v", tc.name, local, remote, ok)
		}
	}
}

func TestHeadsMatchRevParse(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	same := func(stage string) {
		t.Helper()
		l1, r1, err1 := r.heads(t.Context())
		l2, r2, err2 := r.revHeads(t.Context())
		if l1 != l2 || r1 != r2 || err1 != nil || err2 != nil {
			t.Fatalf("%s: heads %q %q %v, rev-parse %q %q %v", stage, l1, r1, err1, l2, r2, err2)
		}
	}
	same("empty")
	writeVault(t, r)
	mustCommit(t, r, "init")
	same("local only")
	head := gitOut(t, r, "rev-parse", "HEAD")
	gitOut(t, r, "update-ref", remoteRef, head)
	same("both")
	gitOut(t, r, "tag", "-a", "-m", "t", "v1", head)
	gitOut(t, r, "update-ref", remoteRef, gitOut(t, r, "rev-parse", "v1"))
	same("remote ref on a tag object")
}
