package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/elliot40404/creds/internal/app"
)

func answer(t *testing.T, msg tea.Msg) setupAns {
	t.Helper()
	ans, ok := msg.(setupAns)
	if !ok {
		t.Fatalf("msg %#v", msg)
	}
	return ans
}

func TestSetupInputDefaultAndEmpty(t *testing.T) {
	s := screenFor(&setupReq{kind: reqInput, prompt: "Repository name", def: "creds-vault"})
	if v := formView(s); !strings.Contains(v, "Repository name") || !strings.Contains(v, "blank keeps creds-vault") {
		t.Fatalf("view %q", v)
	}
	if _, msg := keys(s, enter); answer(t, msg).text != "creds-vault" {
		t.Fatalf("msg %#v", msg)
	}
	s, _ = keys(s, text("mine")...)
	if _, msg := keys(s, enter); answer(t, msg).text != "mine" {
		t.Fatalf("msg %#v", msg)
	}
	empty := screenFor(&setupReq{kind: reqInput, prompt: "Git url of the vault"})
	empty, msg := keys(empty, enter)
	if msg != nil || !strings.Contains(formView(empty), "cannot be empty") {
		t.Fatalf("msg %#v view %q", msg, formView(empty))
	}
	if _, msg := keys(empty, esc); !answer(t, msg).canceled {
		t.Fatalf("msg %#v", msg)
	}
}

func TestSetupPasswordMasked(t *testing.T) {
	s := screenFor(&setupReq{kind: reqPassword, prompt: "New master password"})
	s, _ = keys(s, text(secretA)...)
	v := formView(s)
	if strings.Contains(v, secretA) || !strings.Contains(v, strings.Repeat("*", len(secretA))) {
		t.Fatalf("view %q", v)
	}
	s, _ = s.Update(tea.PasteMsg{Content: "xy"})
	if _, msg := keys(s, enter); answer(t, msg).text != secretA+"xy" {
		t.Fatalf("msg %#v", msg)
	}
}

func TestSetupSelectMoves(t *testing.T) {
	s := screenFor(&setupReq{kind: reqSelect, prompt: "What do you want to do", options: []string{modeNew, modeJoin}})
	if v := formView(s); !strings.Contains(v, modeNew) || !strings.Contains(v, modeJoin) {
		t.Fatalf("view %q", v)
	}
	s, _ = keys(s, down, down)
	ans := answer(t, second(keys(s, enter)))
	if ans.idx != 1 || ans.text != modeJoin {
		t.Fatalf("ans %#v", ans)
	}
	s, _ = keys(s, up)
	if ans := answer(t, second(keys(s, enter))); ans.idx != 0 {
		t.Fatalf("ans %#v", ans)
	}
	if !answer(t, second(keys(s, esc))).canceled {
		t.Fatal("esc should cancel")
	}
}

func second(_ Screen, msg tea.Msg) tea.Msg {
	return msg
}

func TestSetupConfirmKeys(t *testing.T) {
	s := screenFor(&setupReq{kind: reqConfirm, prompt: "Push the vault to git@x:me/v.git?"})
	if v := formView(s); !strings.Contains(v, "Push the vault") {
		t.Fatalf("view %q", v)
	}
	if !answer(t, second(keys(s, ch('y')))).ok {
		t.Fatal("y should confirm")
	}
	for _, k := range []tea.KeyPressMsg{ch('n'), esc, enter} {
		ans := answer(t, second(keys(s, k)))
		if ans.ok || ans.canceled {
			t.Fatalf("key %v ans %#v", k, ans)
		}
	}
	if _, msg := keys(s, ch('z')); msg != nil {
		t.Fatalf("msg %#v", msg)
	}
}

func TestSetupShowRecovery(t *testing.T) {
	code := "aaaa bbbb cccc dddd eeee ffff gggg hhhh iiii jjjj"
	s := screenFor(&setupReq{kind: reqShow, prompt: "Recovery code", text: code})
	v := ansiView(s, 30)
	if !strings.Contains(v, "Recovery code") || !strings.Contains(v, "aaaa bbbb") || !strings.Contains(v, "write it down") {
		t.Fatalf("view %q", v)
	}
	for l := range strings.SplitSeq(v, "\n") {
		if len(l) > 30 {
			t.Fatalf("line %q wider than 30", l)
		}
	}
	if !answer(t, second(keys(s, enter))).ok {
		t.Fatal("enter should continue")
	}
	if !answer(t, second(keys(s, esc))).canceled {
		t.Fatal("esc should cancel")
	}
}

func TestSetupChecksShowFixes(t *testing.T) {
	checks := []app.Check{
		{Name: "git", OK: true, Message: "git at /usr/bin/git"},
		{Name: "clipboard", Message: "no clipboard tool found", Fix: "install wl-copy"},
	}
	s := screenFor(&setupReq{kind: reqChecks, checks: checks})
	v := formView(s)
	for _, want := range []string{"Preflight", "ok", "git at /usr/bin/git", "fix", "no clipboard tool found", "install wl-copy"} {
		if !strings.Contains(v, want) {
			t.Fatalf("view %q missing %q", v, want)
		}
	}
	if !answer(t, second(keys(s, enter))).ok {
		t.Fatal("enter should continue")
	}
}

func TestSetupChecksLabelColumnFitsLongestName(t *testing.T) {
	checks := []app.Check{
		{Name: "git", OK: true, Message: "git found"},
		{Name: "home contents", OK: true, Message: "nothing unexpected"},
	}
	v := formView(screenFor(&setupReq{kind: reqChecks, checks: checks}))
	if !strings.Contains(v, "home contents  nothing unexpected") || !strings.Contains(v, "git            git found") {
		t.Fatalf("view %q", v)
	}
}

func TestSetupEndViews(t *testing.T) {
	done := setupEnd{steps: []string{"creds add -i to add your first entry"}}
	if v := formView(done); !strings.Contains(v, "Setup done") || !strings.Contains(v, "creds add -i") {
		t.Fatalf("view %q", v)
	}
	bad := setupEnd{err: ErrRemoteNoVault}
	if v := formView(bad); !strings.Contains(v, "Setup did not finish") || !strings.Contains(v, "no creds vault") {
		t.Fatalf("view %q", v)
	}
	if len(done.keys()) == 0 || typing(done) {
		t.Fatal("end screen keys")
	}
}

func ansiView(s Screen, width int) string {
	return ansi.Strip(s.View(width, 20))
}

func TestSetupSelectPreselectsCurrent(t *testing.T) {
	s := screenFor(&setupReq{kind: reqSelect, prompt: "Shell", options: []string{"bash", "pwsh"}, cur: 1})
	if ans := answer(t, second(keys(s, enter))); ans.idx != 1 || ans.text != "pwsh" {
		t.Fatalf("ans %#v", ans)
	}
	out := screenFor(&setupReq{kind: reqSelect, prompt: "Shell", options: []string{"bash", "pwsh"}, cur: 7})
	if ans := answer(t, second(keys(out, enter))); ans.idx != 1 {
		t.Fatalf("ans %#v", ans)
	}
}

func TestSetupInputShowsKeptText(t *testing.T) {
	s := screenFor(&setupReq{kind: reqInput, prompt: "Type the recovery code", keep: "aaaa bbbb"})
	v := formView(s)
	if !strings.Contains(v, "aaaa bbbb") || !strings.Contains(v, "Type the recovery code") || strings.Contains(v, "value") {
		t.Fatalf("view %q", v)
	}
}
