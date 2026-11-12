package gitsync

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

const (
	LockStale     = 3 * defaultTimeout
	LockWait      = 2 * time.Minute
	breakSuffix   = ".break"
	holdTries     = 20
	removeTries   = 100
	removeBackoff = 5 * time.Millisecond
	maxLockSize   = 4 << 10
)

var ErrLocked = errors.New("gitsync: sync already running")

type Lock struct {
	path   string
	info   lockInfo
	held   *fsutil.Held
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

type lockInfo struct {
	PID  int       `json:"pid"`
	Time time.Time `json:"time"`
}

func AcquireLock(path string, now time.Time) (*Lock, error) {
	if err := fsutil.EnsureDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	held, err := fsutil.Hold(path+breakSuffix, holdTries)
	if errors.Is(err, fsutil.ErrBusy) {
		return nil, errors.Join(ErrLocked, err)
	}
	if err != nil {
		return nil, err
	}
	if oldHolder(path, now) {
		return nil, errors.Join(ErrLocked, held.Close())
	}
	l := &Lock{path: path, info: lockInfo{PID: os.Getpid(), Time: now.UTC()}, held: held}
	if err := l.write(); err != nil {
		return nil, errors.Join(err, held.Close())
	}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	return l, nil
}

func WaitLock(ctx context.Context, path string, now func() time.Time, every time.Duration, waiting func()) (*Lock, error) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		l, err := AcquireLock(path, now())
		if !errors.Is(err, ErrLocked) && !errors.Is(err, fs.ErrPermission) {
			return l, err
		}
		if waiting != nil {
			waiting()
			waiting = nil
		}
		select {
		case <-ctx.Done():
			return nil, errors.Join(ErrLocked, ctx.Err())
		case <-t.C:
		}
	}
}

func Locked(path string, now time.Time) bool {
	held, err := fsutil.TryHold(path + breakSuffix)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	_ = held.Close()
	return oldHolder(path, now)
}

func oldHolder(path string, now time.Time) bool {
	held, pid, err := readLock(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	if age := now.Sub(held); age >= LockStale || age <= -LockStale {
		return false
	}
	return pid <= 0 || (pid != os.Getpid() && processAlive(pid))
}

func (l *Lock) Context() context.Context {
	return l.ctx
}

func (l *Lock) Release() error {
	var err error
	l.once.Do(func() {
		l.cancel()
		err = errors.Join(l.removeOwn(), l.held.Close())
	})
	return err
}

func (l *Lock) removeOwn() error {
	info, err := readInfo(l.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Time.Equal(l.info.Time) || info.PID != l.info.PID {
		return err
	}
	return remove(l.path)
}

func remove(path string) error {
	var err error
	for range removeTries {
		err = os.Remove(path)
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		time.Sleep(removeBackoff)
	}
	return err
}

func (l *Lock) write() error {
	data, err := json.Marshal(l.info)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(l.path, data)
}

func readLock(path string) (time.Time, int, error) {
	info, err := readInfo(path)
	if err == nil {
		return info.Time, info.PID, nil
	}
	root, rerr := os.OpenRoot(filepath.Dir(path))
	if rerr != nil {
		return time.Time{}, 0, rerr
	}
	defer func() { _ = root.Close() }()
	st, serr := root.Stat(filepath.Base(path))
	if serr != nil {
		return time.Time{}, 0, serr
	}
	return st.ModTime(), 0, nil
}

func readInfo(path string) (lockInfo, error) {
	var info lockInfo
	data, err := fsutil.ReadFileLimit(path, maxLockSize)
	if err != nil {
		return info, err
	}
	return info, json.Unmarshal(data, &info)
}
