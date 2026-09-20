package fsutil

import (
	"io/fs"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func secureTree(dir string) error {
	s, err := sids()
	if err != nil {
		return err
	}
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir || !d.Type().IsRegular() && !d.IsDir() {
			return err
		}
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			return err
		}
		who, err := exposure(sd, s)
		if err != nil || who == "" {
			return err
		}
		inherit := uint32(windows.NO_INHERITANCE)
		if d.IsDir() {
			inherit = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
		}
		return protect(path, inherit)
	})
}
