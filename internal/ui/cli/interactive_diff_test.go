package cli

import (
	"strings"
	"testing"
)

func TestInteractiveDiffSingular(t *testing.T) {
	t.Parallel()
	a, b := joined(t)
	b.editDeploy("make theirs")
	b.ok(&fake{}, "sync")
	a.editDeploy("make mine")
	a.run(&fake{}, "sync")
	r := a.ok(&fake{picks: []string{keepMine}}, "status", "-i")
	if !strings.Contains(r.err, "ops/deploy: 1 difference (mine vs theirs)") {
		t.Fatalf("diff %q", r.err)
	}
}
