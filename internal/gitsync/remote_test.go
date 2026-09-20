package gitsync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/testutil"
)

func newBare(t *testing.T) string {
	t.Helper()
	return testutil.BareRemote(t, Branch)
}

func mustStatus(t *testing.T, r *Repo) Status {
	t.Helper()
	st, err := r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestRemoteAddRemove(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepo(t)
	bare := newBare(t)
	for _, bad := range []string{"", "-uhack", "a\nb"} {
		if err := r.RemoteAdd(ctx, bad); !errors.Is(err, ErrBadRemote) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	if err := r.RemoteRemove(ctx); !errors.Is(err, ErrNoRemote) {
		t.Fatalf("remove none: %v", err)
	}
	if err := r.RemoteAdd(ctx, bare); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoteAdd(ctx, bare); !errors.Is(err, ErrRemoteExists) {
		t.Fatalf("add twice: %v", err)
	}
	if st := mustStatus(t, r); st.Remote != bare {
		t.Fatalf("remote %q", st.Remote)
	}
	if err := r.RemoteRemove(ctx); err != nil {
		t.Fatal(err)
	}
	if st := mustStatus(t, r); st.Remote != "" {
		t.Fatalf("remote %q after remove", st.Remote)
	}
}

func TestStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := newRepo(t)
	if st := mustStatus(t, a); st != (Status{}) || !dirty(t, a) {
		t.Fatalf("fresh %+v", st)
	}
	bare := newBare(t)
	if err := a.RemoteAdd(ctx, bare); err != nil {
		t.Fatal(err)
	}
	writeVault(t, a)
	mustCommit(t, a, "one")
	if st := mustStatus(t, a); st.Ahead != 1 || st.Behind != 0 || dirty(t, a) {
		t.Fatalf("after commit %+v", st)
	}
	gitOut(t, a, "push", "-q", Remote, Branch)
	if st := mustStatus(t, a); st.Ahead != 0 || st.Behind != 0 {
		t.Fatalf("after push %+v", st)
	}
	b := cloneRepo(t, bare)
	writeFile(t, b, "entries/01.enc", ageBlob+"b")
	mustCommit(t, b, "two")
	gitOut(t, b, "push", "-q", Remote, Branch)
	gitOut(t, a, "fetch", "-q", Remote)
	writeFile(t, a, "entries/02.enc", ageBlob+"a")
	if st := mustStatus(t, a); st.Ahead != 0 || st.Behind != 1 || !dirty(t, a) {
		t.Fatalf("behind %+v", st)
	}
}

func dirty(t *testing.T, r *Repo) bool {
	t.Helper()
	return gitOut(t, r, "status", "--porcelain", "--untracked-files=all") != ""
}

func cloneRepo(t *testing.T, bare string) *Repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	if _, err := NewGit(t.TempDir()).Run(context.Background(), "clone", "-q", bare, dir); err != nil {
		t.Fatal(err)
	}
	return Open(dir)
}
