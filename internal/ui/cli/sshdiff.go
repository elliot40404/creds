package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/textdiff"
)

func (e Env) sshDiff(f app.SSHFileStatus) {
	rel := safetext.Line(f.Rel)
	d := f.Diff()
	if d.Key {
		e.note("%s: key differs, content not shown", rel)
		return
	}
	e.note("%s (- here, + vault):", rel)
	for _, l := range d.Lines {
		if l.Op == textdiff.Gap {
			e.note("  ...")
			continue
		}
		e.note("  %c %s", l.Op, safetext.Line(l.Text))
	}
	if d.More > 0 {
		e.note("  ... %d more lines", d.More)
	}
}
