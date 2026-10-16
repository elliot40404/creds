package update

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func holdLikeLoader(t *testing.T, path string) windows.Handle {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func newFile(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, ".creds-new-test")
	if err := os.WriteFile(bin, []byte(newBin), 0o600); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestSwapWhileRunning(t *testing.T) {
	t.Parallel()
	exe := tempExe(t, "creds.exe")
	if err := os.WriteFile(exe+".old", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := holdLikeLoader(t, exe)
	if err := swap(newFile(t, filepath.Dir(exe)), exe); err != nil {
		_ = windows.CloseHandle(h)
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != newBin {
		t.Fatalf("exe %q", got)
	}
	if got := readFile(t, exe+".old"); got != oldBin {
		t.Fatalf("old %q", got)
	}
	if err := windows.CloseHandle(h); err != nil {
		t.Fatal(err)
	}
	removeOld(exe)
	if _, err := os.Stat(exe + ".old"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old kept: %v", err)
	}
}

func TestSwapRollsBack(t *testing.T) {
	t.Parallel()
	exe := tempExe(t, "creds.exe")
	if err := swap(filepath.Join(filepath.Dir(exe), "missing"), exe); err == nil {
		t.Fatal("swap of a missing file worked")
	}
	if got := readFile(t, exe); got != oldBin {
		t.Fatalf("exe %q", got)
	}
	if _, err := os.Stat(exe + ".old"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old kept: %v", err)
	}
}

func TestRemoveOldMissing(t *testing.T) {
	t.Parallel()
	removeOld(filepath.Join(t.TempDir(), "creds.exe"))
}
