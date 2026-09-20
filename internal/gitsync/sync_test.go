package gitsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type pair struct {
	bare string
	a    *Repo
	b    *Repo
}

func newPair(t *testing.T) pair {
	t.Helper()
	bare, a, b := pushedPair(t, func(a *Repo) { writeVault(t, a) }, SyncOptions{})
	return pair{bare: bare, a: a, b: b}
}

func pushedPair(t *testing.T, fill func(a *Repo), opts SyncOptions) (string, *Repo, *Repo) {
	t.Helper()
	bare := newBare(t)
	a := newRepo(t)
	if err := a.RemoteAdd(context.Background(), bare); err != nil {
		t.Fatal(err)
	}
	fill(a)
	mustCommit(t, a, "init")
	if got, err := a.Sync(context.Background(), opts); err != nil || got != Pushed {
		t.Fatalf("initial sync %v %v", got, err)
	}
	return bare, a, cloneRepo(t, bare)
}

func mustSync(t *testing.T, r *Repo) SyncResult {
	t.Helper()
	res, err := r.Sync(context.Background(), SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func commitFile(t *testing.T, r *Repo, name string) {
	t.Helper()
	writeFile(t, r, name, ageBlob+name)
	if !mustCommit(t, r, name) {
		t.Fatal("nothing committed")
	}
}

func bareHead(t *testing.T, bare string) string {
	t.Helper()
	return gitOut(t, Open(bare), "rev-parse", localRef)
}

func TestSyncNoRemote(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	if _, err := r.Sync(context.Background(), SyncOptions{}); !errors.Is(err, ErrNoRemote) {
		t.Fatalf("want ErrNoRemote, got %v", err)
	}
}

func TestSyncEmptyBoth(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	if err := r.RemoteAdd(context.Background(), newBare(t)); err != nil {
		t.Fatal(err)
	}
	if got := mustSync(t, r); got != UpToDate {
		t.Fatalf("got %v", got)
	}
}

func TestSyncWithoutIdentityNeedsUnlockForChanges(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	if got := mustSync(t, p.b); got != UpToDate {
		t.Fatalf("clone sync %v", got)
	}
	commitFile(t, p.a, "identity.pw.age")
	if got := mustSync(t, p.a); got != Pushed {
		t.Fatalf("push %v", got)
	}
	if got := mustSync(t, p.b); got != NeedsUnlock {
		t.Fatalf("sync %v", got)
	}
	if data, err := os.ReadFile(filepath.Join(p.b.Dir, "identity.pw.age")); err != nil || string(data) != ageBlob {
		t.Fatalf("adopted without identity: %q %v", data, err)
	}
	if st := mustStatus(t, p.b); st.Ahead != 0 || st.Behind != 1 {
		t.Fatalf("status %+v", st)
	}
}

func TestSyncDiverged(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	commitFile(t, p.a, "entries/01.enc")
	mustSync(t, p.a)
	want := bareHead(t, p.bare)
	commitFile(t, p.b, "entries/02.enc")
	if _, err := p.b.Sync(context.Background(), SyncOptions{}); !errors.Is(err, ErrDiverged) {
		t.Fatalf("want ErrDiverged, got %v", err)
	}
	if got := bareHead(t, p.bare); got != want {
		t.Fatal("remote changed on divergence")
	}
}

func TestSyncPushRaceBecomesDiverged(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	commitFile(t, p.a, "entries/01.enc")
	pushes := 0
	p.a.hook = func(stage string) {
		if stage != "push" {
			return
		}
		pushes++
		if pushes == 1 {
			commitFile(t, p.b, "entries/02.enc")
			mustSync(t, p.b)
		}
	}
	if _, err := p.a.Sync(context.Background(), SyncOptions{}); !errors.Is(err, ErrDiverged) {
		t.Fatalf("want ErrDiverged, got %v", err)
	}
	if pushes != 1 {
		t.Fatalf("pushes %d", pushes)
	}
}

func TestSyncPushRaceRetried(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		persist bool
		want    error
	}{
		{"recovers", false, nil},
		{"gives up", true, ErrPushRejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newPair(t)
			base := bareHead(t, p.bare)
			other := sideCommit(t, p.bare, base)
			commitFile(t, p.a, "entries/01.enc")
			pushes := 0
			p.a.hook = func(stage string) {
				switch {
				case stage == "fetch" && pushes > 0:
					setBareHead(t, p.bare, base)
				case stage == "push" && (pushes == 0 || tc.persist):
					pushes++
					setBareHead(t, p.bare, other)
				}
			}
			res, err := p.a.Sync(context.Background(), SyncOptions{})
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
			if tc.want == nil && res != Pushed {
				t.Fatalf("result %v", res)
			}
			if tc.persist && pushes != 2 {
				t.Fatalf("pushes %d", pushes)
			}
		})
	}
}

func TestSyncOffline(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	missing := filepath.Join(t.TempDir(), "gone.git")
	if err := r.RemoteAdd(context.Background(), missing); err != nil {
		t.Fatal(err)
	}
	writeVault(t, r)
	mustCommit(t, r, "init")
	_, err := r.Sync(context.Background(), SyncOptions{})
	if _, ok := errors.AsType[*Error](err); !ok {
		t.Fatalf("want git error, got %v", err)
	}
}

func sideCommit(t *testing.T, bare, parent string) string {
	t.Helper()
	g := Open(bare)
	tree := gitOut(t, g, "rev-parse", parent+"^{tree}")
	return gitOut(t, g, "commit-tree", tree, "-p", parent, "-m", "side")
}

func setBareHead(t *testing.T, bare, rev string) {
	t.Helper()
	gitOut(t, Open(bare), "update-ref", localRef, rev)
}
