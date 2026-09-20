//go:build unix

package fsutil

import (
	"fmt"
	"os"
	"syscall"
)

var currentUID = os.Getuid

func CheckPerms(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != currentUID() {
		return fmt.Sprintf("%s is owned by uid %d, not by you (uid %d), run: chown %d %s", path, st.Uid, currentUID(), currentUID(), path), nil
	}
	mode := info.Mode().Perm()
	if mode&0o077 == 0 {
		return "", nil
	}
	want := FilePerm
	if info.IsDir() {
		want = DirPerm
	}
	return fmt.Sprintf("%s is readable by group or others (mode %04o), run: chmod %o %s", path, mode, want, path), nil
}
