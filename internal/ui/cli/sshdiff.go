package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/textdiff"
)

const (
	sshDiffContext = 2
	sshDiffMax     = 40
)

func (e Env) sshDiff(f app.SSHFileStatus) {
	rel := safetext.Line(f.Rel)
	if isSSHKey(f.Disk) || isSSHKey(f.Vault) {
		e.note("%s: key differs, content not shown", rel)
		return
	}
	lines := textdiff.Context(textdiff.Lines(sshText(f.Disk), sshText(f.Vault)), sshDiffContext)
	e.note("%s (- here, + vault):", rel)
	for i, l := range lines {
		if i == sshDiffMax {
			e.note("  ... %d more lines", len(lines)-i)
			return
		}
		if l.Op == textdiff.Gap {
			e.note("  ...")
			continue
		}
		e.note("  %c %s", l.Op, safetext.Line(l.Text))
	}
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
