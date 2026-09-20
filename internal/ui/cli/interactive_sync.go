package cli

import (
	"errors"
	"fmt"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/gitsync"
)

const (
	keepMine     = "keep mine"
	keepTheirs   = "keep theirs"
	skipConflict = "skip"
)

var errNoConflicts = errors.New("no sync conflicts")

func (iv *interview) syncStatus() (app.SyncStatus, error) {
	s, err := iv.service()
	if err != nil {
		return app.SyncStatus{}, err
	}
	return s.SyncStatus()
}

func askResolve(iv *interview) error {
	err := iv.arg(0, func() (string, error) {
		st, err := iv.syncStatus()
		if err != nil || len(st.Conflicts) == 0 {
			return "", firstErr(err, func() error { return errNoConflicts })
		}
		return iv.choose("Conflict", st.Conflicts)
	})
	if err != nil || iv.anyChanged("mine", "theirs") {
		return err
	}
	if err := iv.showDiff(iv.args[0]); err != nil {
		return err
	}
	side, err := iv.choose("Resolve "+iv.args[0], []string{keepMine, keepTheirs})
	if err != nil {
		return err
	}
	flag := map[string]string{keepMine: "mine", keepTheirs: "theirs"}[side]
	return firstErr(iv.set(flag, "true"), func() error { return iv.set("yes", "true") })
}

func askStatus(iv *interview) error {
	if iv.exec(); iv.code != 0 {
		return nil
	}
	st, err := iv.syncStatus()
	if err != nil {
		return err
	}
	if st.Remote == "" {
		return iv.offer("No remote set. Add one?", "remote", "add")
	}
	if len(st.Conflicts) > 0 {
		return iv.resolveAll(st.Conflicts)
	}
	ok, err := iv.p.Confirm("Sync now?")
	if err != nil {
		return err
	}
	if ok {
		iv.run([]string{"sync"}, nil)
		if st, err = iv.syncStatus(); err != nil {
			return err
		}
	}
	return iv.resolveAll(st.Conflicts)
}

func (iv *interview) offer(prompt string, cmd ...string) error {
	ok, err := iv.p.Confirm(prompt)
	if err != nil || !ok {
		return err
	}
	iv.ran = false
	if err := iv.switchTo(cmd); err != nil {
		return err
	}
	iv.exec()
	return nil
}

func (iv *interview) resolveAll(paths []string) error {
	for _, p := range paths {
		if err := iv.showDiff(p); err != nil {
			return err
		}
		side, err := iv.choose("Resolve "+p, []string{keepMine, keepTheirs, skipConflict})
		if err != nil {
			return err
		}
		if side == skipConflict {
			continue
		}
		flag := map[string]string{keepMine: "--mine", keepTheirs: "--theirs"}[side]
		iv.run([]string{"resolve", p, flag, "--yes"}, nil)
	}
	return nil
}

func (iv *interview) showDiff(path string) error {
	s, err := iv.service()
	if err != nil {
		return err
	}
	diffs, err := s.ConflictDiff(path)
	if errors.Is(err, gitsync.ErrNoConflict) {
		return nil
	}
	if err != nil {
		return err
	}
	iv.env.note("%s: %s (mine vs theirs)", path, differences(len(diffs)))
	for _, d := range diffs {
		if d.Secret {
			iv.env.note("  %s: secret changed", d.Name)
			continue
		}
		iv.env.note("  %s: mine %q, theirs %q", d.Name, d.Mine, d.Theirs)
	}
	return nil
}

func differences(n int) string {
	if n == 1 {
		return "1 difference"
	}
	return fmt.Sprintf("%d differences", n)
}
