package app

import (
	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/textdiff"
)

const (
	sshDiffContext = 2
	sshDiffMax     = 40
)

type SSHDiff struct {
	Key   bool
	Lines []textdiff.Line
	More  int
}

func (f SSHFileStatus) Diff() SSHDiff {
	if isSSHKey(f.Disk) || isSSHKey(f.Vault) {
		return SSHDiff{Key: true}
	}
	lines := textdiff.Context(textdiff.Lines(sshText(f.Disk), sshText(f.Vault)), sshDiffContext)
	if len(lines) > sshDiffMax {
		return SSHDiff{Lines: lines[:sshDiffMax], More: len(lines) - sshDiffMax}
	}
	return SSHDiff{Lines: lines}
}

func isSSHKey(f *sshdir.File) bool {
	return f != nil && f.Kind == sshdir.KindPrivateKey
}

func sshText(f *sshdir.File) string {
	if f == nil {
		return ""
	}
	return string(f.Data)
}
