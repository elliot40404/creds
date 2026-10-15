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

type jsonSSHPush struct {
	Dir      string           `json:"dir"`
	Files    []jsonSSHFile    `json:"files"`
	NotRead  []jsonSSHNotRead `json:"not_read"`
	Missing  []string         `json:"missing"`
	Warnings []string         `json:"warnings"`
	Invalid  []string         `json:"invalid"`
}

var sshPushHints = []struct {
	action app.SSHAction
	text   string
}{
	{app.SSHNotAsked, "%d unused keys not pushed, run creds ssh push in a terminal to pick them or pass --all"},
	{app.SSHKeptVault, "%d files changed here and in the vault were not pushed, run creds ssh push -y to overwrite the vault copy"},
	{app.SSHKeptDelete, "%d files deleted here are still in the vault"},
	{app.SSHNeedsPull, "%d files changed in the vault, run creds ssh pull"},
}

func (e Env) printSSHPush(res app.SSHPushResult) error {
	if e.jsonMode() {
		return e.writeJSON(toJSONSSHPush(res))
	}
	e.sshNotes(res.Status)
	counts := map[app.SSHAction]int{}
	for _, f := range res.Files {
		counts[f.Action]++
		if f.Action == app.SSHNoChange {
			continue
		}
		line := fmt.Sprintf("%-12s %s", f.Action, f.Rel)
		if f.Err != nil {
			line += ": " + f.Err.Error()
		}
		if _, err := fmt.Fprintln(e.Out, safetext.Line(line)); err != nil {
			return err
		}
	}
	e.sshPushHints(counts)
	return nil
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

func (e Env) sshPushHints(counts map[app.SSHAction]int) {
	for _, h := range sshPushHints {
		if n := counts[h.action]; n > 0 {
			e.note(h.text, n)
		}
	}
	if counts[app.SSHPushed] == 0 && counts[app.SSHFailed] == 0 {
		e.note("nothing to push")
		return
	}
	e.note("pushed %d files", counts[app.SSHPushed])
}

func toJSONSSHPush(res app.SSHPushResult) jsonSSHPush {
	st := res.Status
	out := jsonSSHPush{Dir: st.Dir, Files: make([]jsonSSHFile, len(res.Files)), NotRead: []jsonSSHNotRead{}, Missing: nonNil(st.Missing)}
	for i, f := range res.Files {
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
