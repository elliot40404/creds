package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func assertProtected(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	ctl, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if ctl&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("%s: dacl not protected", path)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 2 {
		t.Fatalf("%s: dacl %v %v", path, dacl, err)
	}
	if warn, err := CheckPerms(path); err != nil || warn != "" {
		t.Fatalf("%s: warn %q err %v", path, warn, err)
	}
}

func TestWriteFileAtomicProtected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	if err := WriteFileAtomic(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	assertProtected(t, path)
}

func TestWriteFileInProtected(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if err := WriteFileIn(root, "f", []byte("x")); err != nil {
		t.Fatal(err)
	}
	assertProtected(t, filepath.Join(dir, "f"))
}

func TestEnsureDirProtectsCreatedDirs(t *testing.T) {
	top := filepath.Join(t.TempDir(), "home")
	leaf := filepath.Join(top, "vault", "entries")
	if err := EnsureDir(leaf); err != nil {
		t.Fatal(err)
	}
	assertProtected(t, top)
	for _, p := range []string{filepath.Join(top, "vault"), leaf} {
		if warn, err := CheckPerms(p); err != nil || warn != "" {
			t.Fatalf("%s: warn %q err %v", p, warn, err)
		}
	}
}

func TestCheckPermsWarnsOnSharedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := WriteFileAtomic(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	users, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		t.Fatal(err)
	}
	shareWith(t, path, grant(users, windows.NO_INHERITANCE))
	warn, err := CheckPerms(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn, users.String()) || !strings.Contains(warn, "icacls") {
		t.Fatalf("warn %q", warn)
	}
}

func shareWith(t *testing.T, path string, entries ...windows.EXPLICIT_ACCESS) {
	t.Helper()
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		t.Fatal(err)
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, info, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPermsWarnsOnWriteRights(t *testing.T) {
	users, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		t.Fatal(err)
	}
	s, err := sids()
	if err != nil {
		t.Fatal(err)
	}
	for _, mask := range []windows.ACCESS_MASK{windows.WRITE_DAC, windows.WRITE_OWNER, windows.DELETE, windows.FILE_WRITE_DATA, windows.GENERIC_WRITE} {
		path := filepath.Join(t.TempDir(), "f")
		if err := WriteFileAtomic(path, []byte("x")); err != nil {
			t.Fatal(err)
		}
		other := grant(users, windows.NO_INHERITANCE)
		other.AccessPermissions = mask
		shareWith(t, path, grant(s.user, windows.NO_INHERITANCE), other)
		if warn, err := CheckPerms(path); err != nil || !strings.Contains(warn, users.String()) {
			t.Fatalf("mask %#x: warn %q err %v", mask, warn, err)
		}
	}
}

func TestCheckPermsWarnsOnForeignOwner(t *testing.T) {
	path, err := windows.GetSystemWindowsDirectory()
	if err != nil {
		t.Fatal(err)
	}
	warn, err := CheckPerms(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn, "owned by another account") || !strings.Contains(warn, "takeown") {
		t.Fatalf("warn %q", warn)
	}
}

func TestEnsureDirProtectsExistingDir(t *testing.T) {
	dir := t.TempDir()
	users, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		t.Fatal(err)
	}
	s, err := sids()
	if err != nil {
		t.Fatal(err)
	}
	shareWith(t, dir, grant(s.user, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT), grant(users, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT))
	if warn, _ := CheckPerms(dir); warn == "" {
		t.Fatal("setup not shared")
	}
	if err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	assertProtected(t, dir)
}

func TestCreateInProtectedAtCreate(t *testing.T) {
	dir := t.TempDir()
	users, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		t.Fatal(err)
	}
	s, err := sids()
	if err != nil {
		t.Fatal(err)
	}
	shareWith(t, dir, grant(s.user, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT), grant(users, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT))
	g, err := createIn(openRoot(t, dir), "sub")
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	assertProtected(t, filepath.Join(dir, "sub"))
}

func TestSecureTreeFixesSharedFilesInside(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vault")
	if err := EnsureDir(filepath.Join(dir, "entries")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "entries", "00.enc")
	if err := WriteFileAtomic(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	users, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		t.Fatal(err)
	}
	shareWith(t, path, grant(users, windows.NO_INHERITANCE))
	if err := SecureTree(dir); err != nil {
		t.Fatal(err)
	}
	if warn, err := CheckPerms(path); err != nil || warn != "" {
		t.Fatalf("warn %q err %v", warn, err)
	}
}
