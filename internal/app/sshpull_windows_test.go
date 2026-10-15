package app

import (
	"path/filepath"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
	"golang.org/x/sys/windows"
)

func checkSSHPerms(t *testing.T, dir, rel string) {
	t.Helper()
	for _, p := range []string{dir, filepath.Join(dir, filepath.FromSlash(rel))} {
		assertProtectedDACL(t, p)
	}
	if warn, err := fsutil.CheckPerms(filepath.Dir(filepath.Join(dir, filepath.FromSlash(rel)))); err != nil || warn != "" {
		t.Fatalf("warn %q err %v", warn, err)
	}
}

func assertProtectedDACL(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	ctl, _, err := sd.Control()
	if err != nil || ctl&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("%s: dacl not protected (%v)", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 2 {
		t.Fatalf("%s: dacl %v %v", path, dacl, err)
	}
	if warn, err := fsutil.CheckPerms(path); err != nil || warn != "" {
		t.Fatalf("%s: warn %q err %v", path, warn, err)
	}
}
