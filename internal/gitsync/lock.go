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
	removeTries   = 100
	removeBackoff = 5 * time.Millisecond
)

var ErrLocked = errors.New("gitsync: sync already running")

type Lock struct {
	path    string
	info    lockInfo
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelCauseFunc
	err     error
	done    chan struct{}
	stopped chan struct{}
	untick  func()
}

type lockInfo struct {
	PID  int       `json:"pid"`
	Time time.Time `json:"time"`
}

func AcquireLock(path string, now time.Time) (*Lock, error) {
	t := time.NewTicker(lockRefresh)
	l, err := acquire(path, now, t.C)
	if err != nil {
		t.Stop()
		return nil, err
	}
	l.untick = t.Stop
	return l, nil
}

func acquire(path string, now time.Time, tick <-chan time.Time) (*Lock, error) {
	l := &Lock{path: path, info: lockInfo{PID: os.Getpid(), Time: now.UTC()}}
	err := l.create()
	if errors.Is(err, fs.ErrExist) && !Locked(path, now) {
		err = withBreak(path, func() error { return l.takeover(now) })
	}
	if errors.Is(err, fs.ErrExist) {
		return nil, ErrLocked
	}
	if err != nil {
		return nil, err
	}
	l.start(tick)
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
	held, pid, err := readLock(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	if pid > 0 && !processAlive(pid) {
		return false
	}
	age := now.Sub(held)
	return age < LockStale && age > -LockStale
}

func (l *Lock) takeover(now time.Time) error {
	if Locked(l.path, now) {
		return fs.ErrExist
	}
	if err := remove(l.path); err != nil {
		return err
	}
	return l.create()
}

func (l *Lock) Release() error {
	held := l.stop()
	l.mu.Lock()
	defer l.mu.Unlock()
	return errors.Join(held, withBreak(l.path, l.removeOwn))
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

func (l *Lock) create() error {
	data, err := json.Marshal(l.info)
	if err != nil {
		return err
	}
	if err := fsutil.EnsureDir(filepath.Dir(l.path)); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fsutil.FilePerm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return errors.Join(err, os.Remove(l.path))
	}
	return nil
}

func readLock(path string) (time.Time, int, error) {
	info, err := readInfo(path)
	if err == nil {
		return info.Time, info.PID, nil
	}
	st, serr := os.Stat(path)
	if serr != nil {
		return time.Time{}, 0, serr
	}
	return st.ModTime(), 0, nil
}

func readInfo(path string) (lockInfo, error) {
	var info lockInfo
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return info, err
	}
	return info, json.Unmarshal(data, &info)
}
