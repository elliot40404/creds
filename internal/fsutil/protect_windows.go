package fsutil

import (
	"sync"

	"golang.org/x/sys/windows"
)

const fileAllAccess = windows.STANDARD_RIGHTS_ALL | 0x1ff

var sids = sync.OnceValues(loadSIDs)

type sidSet struct {
	user   *windows.SID
	system *windows.SID
	admins *windows.SID
}

func loadSIDs() (sidSet, error) {
	var s sidSet
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return s, err
	}
	if s.user, err = tu.User.Sid.Copy(); err != nil {
		return s, err
	}
	if s.system, err = windows.CreateWellKnownSid(windows.WinLocalSystemSid); err != nil {
		return s, err
	}
	s.admins, err = windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	return s, err
}

func secureDir(path string) error {
	ok, err := private(path)
	if err != nil || ok {
		return err
	}
	return protectDir(path)
}

func protectDir(path string) error {
	return protect(path, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT)
}

func protect(path string, inherit uint32) error {
	s, err := sids()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{grant(s.user, inherit), grant(s.system, inherit)}, nil)
	if err != nil {
		return err
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, info, nil, nil, acl, nil)
}

func grant(sid *windows.SID, inherit uint32) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: fileAllAccess,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inherit,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

func (s sidSet) trusted(sid *windows.SID) bool {
	return sid.Equals(s.user) || sid.Equals(s.system) || sid.Equals(s.admins)
}
