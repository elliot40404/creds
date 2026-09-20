package gitsync

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

const (
	lockRefresh = 20 * time.Second
	lockGrace   = LockStale / 2
)

var ErrLockLost = errors.New("gitsync: lock taken over by another process")

func (l *Lock) Context() context.Context {
	return l.ctx
}

func (l *Lock) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

func (l *Lock) start(tick <-chan time.Time) {
	l.ctx, l.cancel = context.WithCancelCause(context.Background())
	l.done = make(chan struct{})
	l.stopped = make(chan struct{})
	go func() {
		defer close(l.stopped)
		for {
			select {
			case <-l.done:
				return
			case now := <-tick:
				if err := l.refresh(now); errors.Is(err, ErrLockLost) {
					l.lose(err)
					return
				}
			}
		}
	}()
}

func (l *Lock) stop() error {
	if l.done == nil {
		return nil
	}
	close(l.done)
	<-l.stopped
	l.done = nil
	if l.untick != nil {
		l.untick()
	}
	l.cancel(nil)
	return l.Err()
}

func (l *Lock) lose(err error) {
	l.mu.Lock()
	l.err = err
	l.mu.Unlock()
	l.cancel(err)
}

func (l *Lock) refresh(now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.info.Time) > lockGrace {
		return ErrLockLost
	}
	return withBreak(l.path, func() error { return l.rewrite(now) })
}

func (l *Lock) rewrite(now time.Time) error {
	info, err := readInfo(l.path)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrLockLost
	}
	if err != nil {
		return err
	}
	if info.PID != l.info.PID || !info.Time.Equal(l.info.Time) {
		return ErrLockLost
	}
	next := lockInfo{PID: l.info.PID, Time: now.UTC()}
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(l.path, data); err != nil {
		return err
	}
	l.info = next
	return nil
}
