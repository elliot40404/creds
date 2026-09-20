package fsutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWriteFileAtomicRetriesATransientLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.enc")
	fails := 2
	renameIn = func(r *os.Root, old, new string) error {
		if fails > 0 {
			fails--
			return &os.LinkError{Op: "rename", Old: old, New: new, Err: windows.ERROR_SHARING_VIOLATION}
		}
		return r.Rename(old, new)
	}
	t.Cleanup(func() { renameIn = (*os.Root).Rename })
	if err := WriteFileAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); !bytes.Equal(got, []byte("new")) {
		t.Fatalf("got %q", got)
	}
}
