package app

import (
	"crypto/sha256"

	"github.com/elliot40404/creds/internal/sshdir"
)

type SSHChange int

const (
	SSHSame SSHChange = iota + 1
	SSHLocalNew
	SSHRemoteNew
	SSHLocalChanged
	SSHRemoteChanged
	SSHConflict
	SSHLocalDeleted
	SSHRemoteDeleted
)

var sshChangeNames = [...]string{
	SSHSame:          "same",
	SSHLocalNew:      "local new",
	SSHRemoteNew:     "remote new",
	SSHLocalChanged:  "local changed",
	SSHRemoteChanged: "remote changed",
	SSHConflict:      "conflict",
	SSHLocalDeleted:  "local deleted",
	SSHRemoteDeleted: "remote deleted",
}

func (c SSHChange) String() string {
	if c < SSHSame || c > SSHRemoteDeleted {
		return "unknown"
	}
	return sshChangeNames[c]
}

type sshSum struct {
	sum [sha256.Size]byte
	ok  bool
}

func fileSum(f *sshdir.File) sshSum {
	if f == nil {
		return sshSum{}
	}
	return sshSum{sum: f.Sum(), ok: true}
}

func baseSum(st sshdir.State, rel string) sshSum {
	fs, ok := st.Files[rel]
	return sshSum{sum: fs.Sum, ok: ok}
}

func sshCompare(disk, remote, base sshSum) SSHChange {
	switch {
	case disk == remote:
		return SSHSame
	case !base.ok:
		return sshNoBase(disk, remote)
	case disk == base:
		return pickChange(remote.ok, SSHRemoteChanged, SSHRemoteDeleted)
	case remote == base:
		return pickChange(disk.ok, SSHLocalChanged, SSHLocalDeleted)
	}
	return SSHConflict
}

func sshNoBase(disk, remote sshSum) SSHChange {
	switch {
	case !disk.ok:
		return SSHRemoteNew
	case !remote.ok:
		return SSHLocalNew
	}
	return SSHConflict
}

func pickChange(present bool, changed, deleted SSHChange) SSHChange {
	if present {
		return changed
	}
	return deleted
}
