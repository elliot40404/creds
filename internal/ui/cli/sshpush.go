package cli

import (
	"fmt"
	"slices"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/safetext"
)

type jsonSSHFile struct {
	Path   string `json:"path"`
	Change string `json:"change"`
	Action string `json:"action"`
	Error  string `json:"error,omitzero"`
}

type jsonSSHNotRead struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type jsonSSHRun struct {
	Dir      string           `json:"dir"`
	Files    []jsonSSHFile    `json:"files"`
	NotRead  []jsonSSHNotRead `json:"not_read"`
	Missing  []string         `json:"missing"`
	Warnings []string         `json:"warnings"`
	Invalid  []string         `json:"invalid"`
}

type sshHint struct {
	action app.SSHAction
	text   string
}

var sshPushHints = []sshHint{
	{app.SSHNotAsked, "%d unused keys not pushed, run creds ssh push in a terminal to pick them or pass --all"},
	{app.SSHKeptConflict, "%d files changed here and in the vault were not pushed, run creds ssh push -y to overwrite the vault copy"},
	{app.SSHKeptDelete, "%d files deleted here are still in the vault"},
	{app.SSHNeedsPull, "%d files changed in the vault, run creds ssh pull"},
}

func (e Env) printSSHPush(res app.SSHPushResult) error {
	counts, err := e.printSSHFiles(res.Status, res.Files, sshPushHints)
	if err != nil || e.jsonMode() {
		return err
	}
	if counts[app.SSHPushed] == 0 && counts[app.SSHFailed] == 0 {
		e.note("nothing to push")
		return nil
	}
	e.note("pushed %d files", counts[app.SSHPushed])
	return nil
}

func (e Env) printSSHFiles(st app.SSHStatus, files []app.SSHFileResult, hints []sshHint) (map[app.SSHAction]int, error) {
	if e.jsonMode() {
		return nil, e.writeJSON(toJSONSSH(st, files))
	}
	e.sshNotes(st)
	counts := map[app.SSHAction]int{}
	for _, f := range files {
		counts[f.Action]++
		if err := e.printSSHLine(f); err != nil {
			return nil, err
		}
	}
	for _, h := range hints {
		if n := counts[h.action]; n > 0 {
			e.note(h.text, n)
		}
	}
	return counts, nil
}

func (e Env) printSSHLine(f app.SSHFileResult) error {
	if f.Action == app.SSHNoChange {
		return nil
	}
	line := fmt.Sprintf("%-12s %s", f.Action, f.Rel)
	if f.Err != nil {
		line += ": " + f.Err.Error()
	}
	_, err := fmt.Fprintln(e.Out, safetext.Line(line))
	return err
}

func (e Env) sshNotes(st app.SSHStatus) {
	for _, s := range st.Skipped {
		e.note("not read: %s: %s", safetext.Line(s.Rel), safetext.Line(s.Err.Error()))
	}
	for _, m := range st.Missing {
		e.note("warning: ssh config names a missing key file: %s", safetext.Line(m))
	}
	for _, w := range slices.Concat(st.Warnings, st.Invalid) {
		e.note("warning: %s", safetext.Line(w.Error()))
	}
}

func toJSONSSH(st app.SSHStatus, files []app.SSHFileResult) jsonSSHRun {
	out := jsonSSHRun{Dir: st.Dir, Files: make([]jsonSSHFile, len(files)), NotRead: []jsonSSHNotRead{}, Missing: nonNil(st.Missing)}
	for i, f := range files {
		out.Files[i] = jsonSSHFile{Path: f.Rel, Change: f.Change.String(), Action: string(f.Action)}
		if f.Err != nil {
			out.Files[i].Error = f.Err.Error()
		}
	}
	for _, s := range st.Skipped {
		out.NotRead = append(out.NotRead, jsonSSHNotRead{Path: s.Rel, Error: s.Err.Error()})
	}
	out.Warnings, out.Invalid = errStrings(st.Warnings), errStrings(st.Invalid)
	return out
}

func errStrings(errs []error) []string {
	out := make([]string, len(errs))
	for i, err := range errs {
		out[i] = err.Error()
	}
	return out
}
