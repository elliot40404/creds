package tui

import (
	"fmt"
	"strings"

	"github.com/elliot40404/creds/internal/app"
)

const (
	sshChrome   = 3
	sshMaxNotes = 3
	sshChangeW  = 14
)

func (s sshScreen) View(width, height int) string {
	var b strings.Builder
	b.WriteString(line(th.title.Render("ssh files") + "  " + th.dim.Render(s.status.Dir)))
	b.WriteString(line(th.text.Render(sshCount(s.status.Files))))
	b.WriteString(line(""))
	if len(s.status.Files) == 0 {
		b.WriteString(line(th.dim.Render("add one with creds ssh add-host, add-key or keygen")))
		return b.String()
	}
	notes := sshNotes(s.status)
	b.WriteString(s.rows(width, height-sshChrome-len(notes)))
	for _, n := range notes {
		b.WriteString(line(th.warn.Render(n)))
	}
	return b.String()
}

func (s sshScreen) rows(width, rows int) string {
	var b strings.Builder
	files := s.status.Files
	w := widest(files, func(f app.SSHFileStatus) string { return f.Rel })
	start, end := window(s.cursor, rows, len(files))
	for i := start; i < end; i++ {
		f := files[i]
		b.WriteString(pickRow(width, i == s.cursor, func(t theme) string {
			change := t.text
			switch f.Change {
			case app.SSHSame:
				change = t.dim
			case app.SSHConflict:
				change = t.badge
			}
			return t.text.Render(padRight(f.Rel, w)) + "  " + change.Render(padRight(f.Change.String(), sshChangeW)) + "  " + t.dim.Render(sshHint(f))
		}))
	}
	return b.String()
}

func sshCount(files []app.SSHFileStatus) string {
	differ := 0
	for _, f := range files {
		if f.Change != app.SSHSame {
			differ++
		}
	}
	switch {
	case len(files) == 0:
		return "no ssh files here or in the vault"
	case differ == 0:
		return fmt.Sprintf("%d files up to date", len(files))
	}
	return fmt.Sprintf("%d of %d files differ", differ, len(files))
}

func sshHint(f app.SSHFileStatus) string {
	switch {
	case f.Vault == nil && (f.Change == app.SSHRemoteDeleted || f.Change == app.SSHConflict):
		return "deleted in the vault, L deletes it here"
	case f.Change == app.SSHConflict && f.Disk == nil:
		return "deleted here, L takes the vault copy"
	case f.Change == app.SSHConflict:
		return "P keeps this file, L takes the vault copy"
	case asksKey(f):
		return "unused key, p asks"
	}
	switch f.Next() {
	case app.SSHNeedsPush:
		return "p pushes it"
	case app.SSHNeedsPull:
		return "l pulls it"
	case app.SSHSkippedKey:
		return "unused key, skipped, u pushes it"
	case app.SSHKeptDelete:
		return "deleted here, still in the vault"
	}
	return ""
}

func sshNotes(st app.SSHStatus) []string {
	var notes []string
	for _, s := range st.Skipped {
		notes = append(notes, "not read: "+s.Rel+": "+s.Err.Error())
	}
	for _, m := range st.Missing {
		notes = append(notes, "ssh config names a missing key file: "+m)
	}
	for _, w := range st.Warnings {
		notes = append(notes, "warning: "+w.Error())
	}
	for _, w := range st.Invalid {
		notes = append(notes, "warning: "+w.Error())
	}
	if len(notes) > sshMaxNotes {
		more := len(notes) - sshMaxNotes + 1
		notes = append(notes[:sshMaxNotes-1], fmt.Sprintf("%d more notes, run creds ssh status", more))
	}
	return notes
}
