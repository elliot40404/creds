package gitsync

import (
	"context"
	"errors"
)

var ErrLockLost = errors.New("gitsync: lock taken over by another process")

func (l *Lock) Context() context.Context {
	return l.ctx
}

func (l *Lock) Err() error {
	return nil
}
