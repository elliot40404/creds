package gitsync

import (
	"fmt"
	"io"
	"strconv"

	"github.com/elliot40404/creds/internal/fsutil"
)

const (
	maxBlobTotal  = 256 << 20
	maxTreeOutput = 1 << 20
	maxCommitWalk = 10000
)

var walkLimit = strconv.Itoa(maxCommitWalk)

func tooLarge(what string) error {
	return fmt.Errorf("gitsync: %s %w", what, fsutil.ErrTooLarge)
}

type limitWriter struct {
	w    io.Writer
	left int64
	what string
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > l.left {
		return 0, tooLarge(l.what)
	}
	l.left -= int64(len(p))
	return l.w.Write(p)
}
