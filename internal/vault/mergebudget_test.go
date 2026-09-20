package vault

import (
	"errors"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
)

func TestMergeOpenRejectsOverBudget(t *testing.T) {
	t.Parallel()
	m := merger{left: 4}
	if _, err := m.open(0, []byte("12345")); !errors.Is(err, fsutil.ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeOpenSpendsTheBudget(t *testing.T) {
	t.Parallel()
	m := merger{left: 8}
	if _, err := m.open(0, []byte("12345")); errors.Is(err, fsutil.ErrTooLarge) {
		t.Fatalf("budget refused a bucket that fits: %v", err)
	}
	if m.left != 3 {
		t.Fatalf("left = %d", m.left)
	}
}
