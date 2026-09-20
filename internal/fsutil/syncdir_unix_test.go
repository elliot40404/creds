//go:build unix

package fsutil

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWriteFileAtomicIgnoresUnsupportedDirSync(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EINVAL, syscall.ENOTSUP} {
		syncDirFile = func(f *os.File) error { return &os.PathError{Op: "sync", Path: f.Name(), Err: errno} }
		err := WriteFileAtomic(filepath.Join(t.TempDir(), "f"), []byte("x"))
		syncDirFile = (*os.File).Sync
		if err != nil {
			t.Fatalf("%v: %v", errno, err)
		}
	}
}
