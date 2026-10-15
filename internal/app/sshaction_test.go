package app

import "testing"

func TestSSHNextAction(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		f    SSHFileStatus
		want SSHAction
	}{
		"same":               {SSHFileStatus{Change: SSHSame}, SSHNoChange},
		"local new":          {SSHFileStatus{Change: SSHLocalNew}, SSHNeedsPush},
		"unused key":         {SSHFileStatus{Change: SSHLocalNew, Unused: true}, SSHNeedsPush},
		"skipped unused key": {SSHFileStatus{Change: SSHLocalNew, Unused: true, Skipped: true}, SSHSkippedKey},
		"skipped but used":   {SSHFileStatus{Change: SSHLocalNew, Skipped: true}, SSHNeedsPush},
		"local changed":      {SSHFileStatus{Change: SSHLocalChanged}, SSHNeedsPush},
		"remote new":         {SSHFileStatus{Change: SSHRemoteNew}, SSHNeedsPull},
		"remote changed":     {SSHFileStatus{Change: SSHRemoteChanged}, SSHNeedsPull},
		"remote deleted":     {SSHFileStatus{Change: SSHRemoteDeleted}, SSHNeedsPull},
		"conflict":           {SSHFileStatus{Change: SSHConflict}, SSHKeptConflict},
		"local deleted":      {SSHFileStatus{Change: SSHLocalDeleted}, SSHKeptDelete},
	}
	for name, c := range cases {
		if got := c.f.Next(); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}
