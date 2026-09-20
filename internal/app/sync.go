package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elliot40404/creds/internal/anchor"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/vault"
)

const lockPoll = 200 * time.Millisecond

type ConflictError struct {
	Paths []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%d sync conflicts", len(e.Paths))
}

type SyncStatus struct {
	Remote     string
	Ahead      int
	Behind     int
	LastSync   time.Time
	LastResult string
	LastError  string
	Conflicts  []string
	Trusted    string
	Generation uint64
}

func (s *Service) RemoteAdd(url string) error {
	ctx := context.Background()
	r, err := s.syncRepo(ctx)
	if err != nil {
		return err
	}
	return r.RemoteAdd(ctx, url)
}

func (s *Service) RemoteRemove() error {
	ctx := context.Background()
	r, err := s.syncRepo(ctx)
	if err != nil {
		return err
	}
	return r.RemoteRemove(ctx)
}

func (s *Service) SyncStatus() (SyncStatus, error) {
	ctx := context.Background()
	r, err := s.syncRepo(ctx)
	if err != nil {
		return SyncStatus{}, err
	}
	gst, err := r.Status(ctx)
	if err != nil {
		return SyncStatus{}, err
	}
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil {
		return SyncStatus{}, err
	}
	a, err := anchor.Load(s.Paths.Anchor())
	if err != nil && !errors.Is(err, anchor.ErrNoAnchor) {
		return SyncStatus{}, err
	}
	return SyncStatus{
		Trusted:    shortID(a.Manifest),
		Generation: a.Generation,
		Remote:     gst.Remote,
		Ahead:      gst.Ahead,
		Behind:     gst.Behind,
		LastSync:   st.LastSync,
		LastResult: st.LastResult,
		LastError:  st.LastError,
		Conflicts:  conflictPaths(st.Pending()),
	}, nil
}

func (s *Service) Sync() (string, error) {
	return s.lockedSync(nil)
}

func (s *Service) Resolve(path string, side vault.Side) (string, error) {
	return s.lockedSync(func(sy *gitsync.Syncer) error {
		if err := sy.Resolve(path, side); err != nil {
			return err
		}
		return s.pendingError()
	})
}

const (
	quietLockPoll   = 250 * time.Millisecond
	quietLockBudget = 5 * time.Second
)

func (s *Service) SyncQuiet() (err error) {
	id, _ := s.sessionIdentity()
	after := s.CurrentConfig().Sync.After
	ctx, cancel := context.WithTimeout(context.Background(), after+gitsync.LockWait)
	defer cancel()
	r, err := s.remoteRepo(ctx)
	if err != nil {
		return err
	}
	if err := gitsync.Wait(ctx, after); err != nil {
		return err
	}
	budget, stop := context.WithTimeout(ctx, quietLockBudget)
	defer stop()
	lock, err := gitsync.WaitLock(budget, s.Paths.SyncLock(), s.now, quietLockPoll, nil)
	if err != nil {
		if errors.Is(err, gitsync.ErrLocked) {
			return nil
		}
		return err
	}
	defer func() { err = errors.Join(err, lock.Release()) }()
	sy := s.newSyncer(r, id)
	if _, err = sy.Sync(lock.Context()); err != nil || !s.stillPending() {
		return err
	}
	_, err = sy.Sync(lock.Context())
	return err
}

func (s *Service) stillPending() bool {
	st, err := gitsync.LoadState(s.Paths.State())
	return err == nil && !st.PendingSince.IsZero()
}

func (s *Service) lockedSync(before func(*gitsync.Syncer) error) (string, error) {
	sy, err := s.syncer()
	if err != nil {
		return "", err
	}
	var res string
	err = s.withLock(func(lock *gitsync.Lock) (bool, error) {
		if before != nil {
			if err := before(sy); err != nil {
				return false, err
			}
		}
		result, err := sy.Sync(lock.Context())
		if errors.Is(err, gitsync.ErrConflict) {
			if perr := s.pendingError(); perr != nil {
				return false, perr
			}
		}
		res = result.String()
		return false, err
	})
	if err != nil {
		return "", err
	}
	return res, nil
}

func (s *Service) withLock(fn func(*gitsync.Lock) (bool, error)) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitsync.LockWait)
	defer cancel()
	lock, err := gitsync.WaitLock(ctx, s.Paths.SyncLock(), s.now, lockPoll, s.Waiting)
	if err != nil {
		return err
	}
	changed, err := fn(lock)
	if rerr := lock.Release(); !errors.Is(err, gitsync.ErrLockLost) {
		err = errors.Join(err, rerr)
	}
	if err != nil {
		return err
	}
	if changed {
		s.afterWrite()
	}
	return nil
}

func (s *Service) pendingError() error {
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil {
		return err
	}
	if left := st.Pending(); len(left) > 0 {
		return &ConflictError{Paths: conflictPaths(left)}
	}
	return nil
}

func (s *Service) syncer() (*gitsync.Syncer, error) {
	r, err := s.remoteRepo(context.Background())
	if err != nil {
		return nil, err
	}
	id, err := s.identity()
	if err != nil {
		return nil, err
	}
	return s.newSyncer(r, id), nil
}

func (s *Service) newSyncer(r *gitsync.Repo, id *crypto.Identity) *gitsync.Syncer {
	return &gitsync.Syncer{Repo: r, StatePath: s.Paths.State(), AnchorPath: s.Paths.Anchor(), Identity: id, Now: s.Now}
}

func (s *Service) remoteRepo(ctx context.Context) (*gitsync.Repo, error) {
	r, err := s.syncRepo(ctx)
	if err != nil {
		return nil, err
	}
	url, err := r.RemoteURL(ctx)
	if err != nil {
		return nil, err
	}
	if url == "" {
		return nil, gitsync.ErrNoRemote
	}
	return r, nil
}

func (s *Service) syncRepo(ctx context.Context) (*gitsync.Repo, error) {
	if err := s.requireVault(); err != nil {
		return nil, err
	}
	return s.repo(ctx)
}

func conflictPaths(pending []gitsync.PendingConflict) []string {
	var out []string
	for _, c := range pending {
		if len(c.Paths) > 0 {
			out = append(out, c.Paths...)
		} else {
			out = append(out, c.File)
		}
	}
	return out
}
