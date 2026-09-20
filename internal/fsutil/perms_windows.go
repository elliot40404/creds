package fsutil

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	fileDeleteChild = 0x40
	riskyMask       = windows.FILE_READ_DATA | windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | fileDeleteChild |
		windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER |
		windows.GENERIC_READ | windows.GENERIC_WRITE | windows.GENERIC_ALL
	allowedCompoundAce       = 0x4
	allowedObjectAce         = 0x5
	allowedCallbackAce       = 0x9
	allowedCallbackObjectAce = 0xb
	unknownAce               = "unknown access entry"
)

func CheckPerms(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	s, err := sids()
	if err != nil {
		return "", err
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return "", err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return "", err
	}
	if !s.trusted(owner) {
		return fmt.Sprintf("%s is owned by another account (%s), run: takeown /f \"%s\"", path, owner.String(), path), nil
	}
	who, err := exposure(sd, s)
	if err != nil || who == "" {
		return "", err
	}
	return permWarning(path, who, s.user, info.IsDir()), nil
}

func private(path string) (bool, error) {
	s, err := sids()
	if err != nil {
		return false, err
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	ctl, _, err := sd.Control()
	if err != nil || ctl&windows.SE_DACL_PROTECTED == 0 {
		return false, err
	}
	who, err := exposure(sd, s)
	return who == "", err
}

func exposure(sd *windows.SECURITY_DESCRIPTOR, s sidSet) (string, error) {
	dacl, _, err := sd.DACL()
	if err != nil {
		return "", err
	}
	if dacl == nil {
		return "everyone", nil
	}
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return "", err
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Mask&riskyMask == 0 {
			continue
		}
		switch ace.Header.AceType {
		case windows.ACCESS_ALLOWED_ACE_TYPE, allowedCallbackAce:
			sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
			if !s.trusted(sid) {
				return sid.String(), nil
			}
		case allowedCompoundAce, allowedObjectAce, allowedCallbackObjectAce:
			return unknownAce, nil
		}
	}
	return "", nil
}

func permWarning(path, who string, user *windows.SID, dir bool) string {
	perm := "F"
	if dir {
		perm = "(OI)(CI)F"
	}
	return fmt.Sprintf("%s is open to other accounts (%s), run: icacls \"%s\" /inheritance:r /grant:r *%s:%s *S-1-5-18:%s", path, who, path, user.String(), perm, perm)
}
