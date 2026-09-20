package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/testutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func bareRemote(t *testing.T) string {
	t.Helper()
	return testutil.BareRemote(t, gitsync.Branch)
}

func pushFiles(t *testing.T, remote string, names ...string) {
	t.Helper()
	testutil.PushChange(t, remote, gitsync.Branch, "seed", func(work string) {
		for _, name := range names {
			path := filepath.Join(work, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestProbeRemoteStates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files []string
		want  RemoteState
	}{
		{"empty", nil, RemoteEmpty},
		{"vault", []string{vaultfiles.MetaFile, vaultfiles.PasswordFile, "entries/00.enc"}, RemoteVault},
		{"other", []string{"README.md"}, RemoteOther},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, _, _ := newService(t)
			remote := bareRemote(t)
			if len(tc.files) > 0 {
				pushFiles(t, remote, tc.files...)
			}
			got, err := s.Setup().ProbeRemote(context.Background(), remote)
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			if got != tc.want {
				t.Fatalf("state = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProbeRemoteUnreachable(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	missing := filepath.Join(t.TempDir(), "gone.git")
	if _, err := s.Setup().ProbeRemote(context.Background(), missing); err == nil {
		t.Fatal("want error")
	}
}

func TestSetupRemoteErrNamesMissingRepo(t *testing.T) {
	t.Parallel()
	s, _, _ := newService(t)
	missing := filepath.Join(t.TempDir(), "gone.git")
	err := SetupRemoteErr(s.Setup().CheckRemoteEmpty(context.Background(), missing))
	if !errors.Is(err, errRemoteMissing) || !RemoteRetryable(err) {
		t.Fatalf("err %v", err)
	}
	if other := errors.New("boom"); !errors.Is(SetupRemoteErr(other), other) || RemoteRetryable(other) {
		t.Fatal("other errors must pass through and not retry")
	}
}

func TestReplaceRemotePushes(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	remote := bareRemote(t)
	if err := s.Setup().ReplaceRemote(remote); err != nil {
		t.Fatalf("use: %v", err)
	}
	st, err := s.SyncStatus()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Remote != remote || st.Ahead != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestCheckRemoteEmptyRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files []string
		want  error
	}{
		{"vault", []string{vaultfiles.MetaFile, vaultfiles.PasswordFile}, ErrRemoteHasVault},
		{"junk", []string{"README.md"}, ErrRemoteNotEmpty},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, _, _, _ := initVault(t)
			remote := bareRemote(t)
			pushFiles(t, remote, tc.files...)
			if err := s.Setup().CheckRemoteEmpty(context.Background(), remote); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			st, err := s.SyncStatus()
			if err != nil {
				t.Fatal(err)
			}
			if st.Remote != "" {
				t.Fatalf("remote kept after refusal: %s", st.Remote)
			}
		})
	}
}

func TestCheckRemoteEmptyUsesInjectedProbe(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	u := s.Setup()
	u.Probe = func(context.Context, string) (RemoteState, error) { return RemoteVault, nil }
	if err := u.CheckRemoteEmpty(context.Background(), "https://example.invalid/x.git"); !errors.Is(err, ErrRemoteHasVault) {
		t.Fatalf("err = %v", err)
	}
}
