package gitsync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vault"
)

type State struct {
	LastPull   time.Time         `json:"last_pull,omitzero"`
	LastSync   time.Time         `json:"last_sync,omitzero"`
	LastResult string            `json:"last_result,omitzero"`
	LastError  string            `json:"last_error,omitzero"`
	Conflicts  []PendingConflict `json:"conflicts,omitzero"`

	PendingSince time.Time `json:"pending_since,omitzero"`
	LastSpawn    time.Time `json:"last_spawn,omitzero"`

	PasswordChanged time.Time `json:"password_changed,omitzero"`

	ClearError string `json:"clear_error,omitzero"`
}

type PendingConflict struct {
	File   string      `json:"file,omitzero"`
	Paths  []string    `json:"paths,omitzero"`
	IDs    []uuid.UUID `json:"ids,omitzero"`
	Choice vault.Side  `json:"choice,omitzero"`
}

const (
	stateLockSuffix = ".lock"
	maxStateSize    = 1 << 20
)

var (
	ErrNoConflict = errors.New("gitsync: no pending conflict for path")
	ErrBadSide    = errors.New("gitsync: invalid resolution side")
)

type Syncer struct {
	Repo       *Repo
	StatePath  string
	AnchorPath string
	Identity   *crypto.Identity
	Now        func() time.Time
}

func LoadState(path string) (State, error) {
	var s State
	err := fsutil.ReadJSON(path, maxStateSize, &s)
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("gitsync: parse state: %w", err)
	}
	return s, nil
}

func SaveState(path string, s State) error {
	return fsutil.WriteJSON(path, s)
}

func UpdateState(path string, fn func(*State) (bool, error)) (State, error) {
	var st State
	err := fsutil.WithFileLock(path+stateLockSuffix, func() error {
		var err error
		if st, err = LoadState(path); err != nil {
			return err
		}
		changed, err := fn(&st)
		if err != nil || !changed {
			return err
		}
		return SaveState(path, st)
	})
	return st, err
}

func (s *Syncer) Sync(ctx context.Context) (SyncResult, error) {
	st, err := LoadState(s.StatePath)
	if err != nil {
		return UpToDate, err
	}
	start := s.now().UTC()
	before := s.Repo.passwordSum()
	opts := st.options(s.Identity)
	opts.AnchorPath = s.AnchorPath
	res, err := s.Repo.Sync(ctx, opts)
	if cerr := context.Cause(ctx); cerr != nil {
		return res, errors.Join(err, cerr)
	}
	changed := s.Repo.passwordSum() != before
	_, saveErr := UpdateState(s.StatePath, func(st *State) (bool, error) {
		pending := st.PendingSince
		st.record(s.now(), res, err)
		if pending.After(start) {
			st.PendingSince = pending
		}
		if changed {
			st.PasswordChanged = s.now().UTC()
		}
		return true, nil
	})
	return res, errors.Join(err, saveErr)
}

func (s *Syncer) Resolve(path string, side vault.Side) error {
	if side != vault.Mine && side != vault.Theirs {
		return ErrBadSide
	}
	_, err := UpdateState(s.StatePath, func(st *State) (bool, error) {
		found := false
		for i, c := range st.Conflicts {
			if c.File == path || slices.Contains(c.Paths, path) {
				st.Conflicts[i].Choice = side
				found = true
			}
		}
		if !found {
			return false, fmt.Errorf("%w: %s", ErrNoConflict, path)
		}
		return true, nil
	})
	return err
}

func (st State) Pending() []PendingConflict {
	return st.filter(false)
}

func (st State) filter(chosen bool) []PendingConflict {
	var out []PendingConflict
	for _, c := range st.Conflicts {
		if (c.Choice != 0) == chosen {
			out = append(out, c)
		}
	}
	return out
}

func (st State) stillChosen(live []PendingConflict) []PendingConflict {
	var out []PendingConflict
	for _, c := range st.filter(true) {
		if slices.ContainsFunc(live, c.same) {
			out = append(out, c)
		}
	}
	return out
}

func (c PendingConflict) same(o PendingConflict) bool {
	if c.File != "" {
		return c.File == o.File
	}
	return slices.ContainsFunc(c.IDs, func(id uuid.UUID) bool { return slices.Contains(o.IDs, id) })
}

func (st State) options(id *crypto.Identity) SyncOptions {
	opts := SyncOptions{Identity: id, PreferFile: map[string]vault.Side{}, PreferEntry: map[uuid.UUID]vault.Side{}}
	for _, c := range st.Conflicts {
		if c.Choice == 0 {
			continue
		}
		if c.File != "" {
			opts.PreferFile[c.File] = c.Choice
		}
		for _, id := range c.IDs {
			opts.PreferEntry[id] = c.Choice
		}
	}
	return opts
}

func (s *Syncer) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (st *State) record(now time.Time, res SyncResult, err error) {
	st.LastSync = now.UTC()
	st.LastError = ""
	cerr, conflicted := errors.AsType[*ConflictError](err)
	switch {
	case conflicted:
		st.LastPull = st.LastSync
		st.LastResult = "conflict"
		st.Conflicts = append(st.stillChosen(pending(cerr.Live)), pending(cerr.Conflicts)...)
	case err != nil:
		st.LastResult = "error"
		st.LastError = err.Error()
	case res == NeedsUnlock:
		st.LastResult = res.String()
	default:
		st.LastPull = st.LastSync
		st.LastResult = res.String()
		st.Conflicts = nil
		st.PendingSince = time.Time{}
	}
}

func MarkPending(path string, now time.Time) (State, error) {
	return UpdateState(path, func(st *State) (bool, error) {
		if !st.PendingSince.IsZero() {
			return false, nil
		}
		st.PendingSince = now.UTC()
		return true, nil
	})
}

func MarkSpawn(path string, now time.Time) error {
	_, err := UpdateState(path, func(st *State) (bool, error) {
		st.LastSpawn = now.UTC()
		return true, nil
	})
	return err
}

func pending(conflicts []Conflict) []PendingConflict {
	out := make([]PendingConflict, 0, len(conflicts))
	for _, c := range conflicts {
		p := PendingConflict{File: c.File}
		if e := c.Entry; e != nil {
			p.Paths = slices.Clone(e.Paths)
			p.IDs = conflictIDs(c)
		}
		out = append(out, p)
	}
	return out
}

func conflictIDs(c Conflict) []uuid.UUID {
	var ids []uuid.UUID
	add := func(id uuid.UUID) {
		if id != uuid.Nil() && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	add(c.Entry.ID)
	if c.Entry.Ours != nil {
		add(c.Entry.Ours.ID)
	}
	if c.Entry.Theirs != nil {
		add(c.Entry.Theirs.ID)
	}
	return ids
}

func (r SyncResult) String() string {
	switch r {
	case Pushed:
		return "pushed"
	case FastForwarded:
		return "fast-forwarded"
	case Merged:
		return "merged"
	case NeedsUnlock:
		return "needs unlock"
	}
	return "up-to-date"
}
