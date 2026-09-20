package fsutil

import (
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var fileSD = sync.OnceValues(func() (*windows.SECURITY_DESCRIPTOR, error) {
	s, err := sids()
	if err != nil {
		return nil, err
	}
	return windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + s.user.String() + ")(A;;FA;;;SY)")
})

func createIn(root *os.Root, name string) (*os.File, error) {
	d, err := root.Open(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer func() { _ = d.Close() }()
	return createAt(d, filepath.Base(name))
}

func createAt(dir *os.File, base string) (*os.File, error) {
	sd, err := fileSD()
	if err != nil {
		return nil, err
	}
	name, err := windows.NewNTUnicodeString(base)
	if err != nil {
		return nil, err
	}
	oa := windows.OBJECT_ATTRIBUTES{
		RootDirectory:      windows.Handle(dir.Fd()),
		ObjectName:         name,
		Attributes:         windows.OBJ_CASE_INSENSITIVE,
		SecurityDescriptor: sd,
	}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var h windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&h, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.SYNCHRONIZE, &oa, &iosb, nil,
		windows.FILE_ATTRIBUTE_NORMAL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, windows.FILE_CREATE,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT, 0, 0)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: filepath.Join(dir.Name(), base), Err: err}
	}
	return os.NewFile(uintptr(h), filepath.Join(dir.Name(), base)), nil
}
