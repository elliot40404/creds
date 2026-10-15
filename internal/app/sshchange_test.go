package app

import (
	"crypto/sha256"
	"testing"
)

func TestSSHCompareEveryCase(t *testing.T) {
	t.Parallel()
	a := sshSum{sum: sha256.Sum256([]byte("a")), ok: true}
	b := sshSum{sum: sha256.Sum256([]byte("b")), ok: true}
	c := sshSum{sum: sha256.Sum256([]byte("c")), ok: true}
	none := sshSum{}
	cases := map[string]struct {
		disk, remote, base sshSum
		want               SSHChange
	}{
		"same":                       {a, a, a, SSHSame},
		"same no base":               {a, a, none, SSHSame},
		"same both moved from base":  {b, b, a, SSHSame},
		"local new":                  {a, none, none, SSHLocalNew},
		"remote new":                 {none, a, none, SSHRemoteNew},
		"local changed":              {b, a, a, SSHLocalChanged},
		"remote changed":             {a, b, a, SSHRemoteChanged},
		"local deleted":              {none, a, a, SSHLocalDeleted},
		"remote deleted":             {a, none, a, SSHRemoteDeleted},
		"conflict both changed":      {b, c, a, SSHConflict},
		"conflict no base":           {a, b, none, SSHConflict},
		"deleted here changed there": {none, b, a, SSHConflict},
		"changed here deleted there": {b, none, a, SSHConflict},
	}
	for name, tc := range cases {
		if got := sshCompare(tc.disk, tc.remote, tc.base); got != tc.want {
			t.Errorf("%s: got %s, want %s", name, got, tc.want)
		}
	}
}

func TestSSHChangeNames(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for c := SSHSame; c <= SSHRemoteDeleted; c++ {
		name := c.String()
		if name == "" || name == "unknown" || seen[name] {
			t.Fatalf("change %d has name %q", c, name)
		}
		seen[name] = true
	}
	for _, c := range []SSHChange{0, SSHRemoteDeleted + 1, -1} {
		if c.String() != "unknown" {
			t.Errorf("change %d: %q", c, c.String())
		}
	}
}
