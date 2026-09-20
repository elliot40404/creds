package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func sstep(t *testing.T, m setupModel, msg tea.Msg) setupModel {
	t.Helper()
	next, cmd := m.Update(msg)
	got, ok := next.(setupModel)
	if !ok {
		t.Fatalf("model %#v", next)
	}
	if _, key := msg.(tea.KeyPressMsg); !key || cmd == nil {
		return got
	}
	if ans, ok := cmd().(setupAns); ok {
		return sstep(t, got, ans)
	}
	return got
}

func frameOf(m setupModel) string {
	return ansi.Strip(m.frame())
}

func newModel(t *testing.T) setupModel {
	t.Helper()
	m := newSetup(SetupOptions{Setup: newSetupUnit(t, &scripted{t: t})})
	return sstep(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
}

func TestModelAsksAndAnswers(t *testing.T) {
	m := newModel(t)
	req := &setupReq{kind: reqSelect, prompt: "What do you want to do", options: []string{modeNew, modeJoin}, reply: make(chan setupAns, 1)}
	m = sstep(t, m, setupAskMsg{req})
	if !strings.Contains(frameOf(m), modeJoin) {
		t.Fatalf("frame %q", frameOf(m))
	}
	m = sstep(t, sstep(t, m, down), enter)
	ans := <-req.reply
	if ans.idx != 1 || ans.canceled {
		t.Fatalf("ans %#v", ans)
	}
	if !strings.Contains(frameOf(m), "working") {
		t.Fatalf("frame %q", frameOf(m))
	}
}

func TestModelHidesPassword(t *testing.T) {
	m := newModel(t)
	req := &setupReq{kind: reqPassword, prompt: "New master password", reply: make(chan setupAns, 1)}
	m = sstep(t, m, setupAskMsg{req})
	for _, k := range text(secretA) {
		m = sstep(t, m, k)
	}
	if strings.Contains(frameOf(m), secretA) {
		t.Fatalf("frame leaks the password: %q", frameOf(m))
	}
	sstep(t, m, enter)
	if ans := <-req.reply; ans.text != secretA {
		t.Fatalf("ans %#v", ans)
	}
}

func TestModelNotesAndDone(t *testing.T) {
	m := newModel(t)
	m = sstep(t, m, setupNoteMsg{msg: "vault created"})
	m = sstep(t, m, setupNoteMsg{msg: "bad recovery code, try again", warn: true})
	f := frameOf(m)
	if !strings.Contains(f, "vault created") || !strings.Contains(f, "warning: bad recovery code") {
		t.Fatalf("frame %q", f)
	}
	m = sstep(t, m, setupDoneMsg{})
	f = frameOf(m)
	if !strings.Contains(f, "Setup done") || !strings.Contains(f, "creds add -i") {
		t.Fatalf("frame %q", f)
	}
	if m.quit {
		t.Fatal("should wait for a key before quitting")
	}
	if end := sstep(t, m, enter); !end.quit || end.err != nil {
		t.Fatalf("quit %v err %v", end.quit, end.err)
	}
}

func TestModelFailureShowsHint(t *testing.T) {
	m := sstep(t, newModel(t), setupDoneMsg{ErrRemoteNoVault})
	if f := frameOf(m); !strings.Contains(f, "Setup did not finish") || !strings.Contains(f, "no creds vault") {
		t.Fatalf("frame %q", f)
	}
	if !errors.Is(m.err, ErrRemoteNoVault) {
		t.Fatalf("err %v", m.err)
	}
}

func TestModelCancelReplies(t *testing.T) {
	m := newModel(t)
	req := &setupReq{kind: reqInput, prompt: "Git url of the vault", reply: make(chan setupAns, 1)}
	m = sstep(t, m, setupAskMsg{req})
	m = sstep(t, m, ctrl('c'))
	if ans := <-req.reply; !ans.canceled {
		t.Fatalf("ans %#v", ans)
	}
	m = sstep(t, m, setupDoneMsg{errSetupCanceled})
	if !m.quit {
		t.Fatal("cancel should quit")
	}
}

func TestModelFrameFits(t *testing.T) {
	m := newModel(t)
	m = sstep(t, m, setupAskMsg{&setupReq{kind: reqChecks, checks: newSetupUnit(t, &scripted{t: t}).Preflight(), reply: make(chan setupAns, 1)}})
	m = sstep(t, m, setupNoteMsg{msg: strings.Repeat("long note ", 20)})
	lines := strings.Split(frameOf(m), "\n")
	if len(lines) > 24 {
		t.Fatalf("frame has %d lines", len(lines))
	}
	for _, l := range lines {
		if ansi.StringWidth(l) > 80 {
			t.Fatalf("line %q is wider than 80", l)
		}
	}
	small := sstep(t, m, tea.WindowSizeMsg{Width: 20, Height: 6})
	if !strings.Contains(frameOf(small), "window too small") {
		t.Fatalf("frame %q", frameOf(small))
	}
}

func TestModelInputAsksByQuestion(t *testing.T) {
	m := newModel(t)
	req := &setupReq{kind: reqInput, prompt: "Idle timeout", def: "15m", reply: make(chan setupAns, 1)}
	m = sstep(t, m, setupAskMsg{req})
	f := frameOf(m)
	if !strings.Contains(f, "Idle timeout") || strings.Contains(f, "value") {
		t.Fatalf("frame %q", f)
	}
}

func TestModelKeepsShownCodeOnRetype(t *testing.T) {
	code := "aaaa bbbb cccc dddd"
	m := newModel(t)
	m = sstep(t, m, setupAskMsg{&setupReq{kind: reqShow, prompt: "Recovery code", text: code, reply: make(chan setupAns, 1)}})
	m = sstep(t, m, enter)
	retype := &setupReq{kind: reqInput, prompt: "Type the recovery code", reply: make(chan setupAns, 1)}
	m = sstep(t, m, setupAskMsg{retype})
	if !strings.Contains(frameOf(m), code) {
		t.Fatalf("code not kept: %q", frameOf(m))
	}
	m = sstep(t, m, setupAskMsg{&setupReq{kind: reqInput, prompt: "Type the recovery code", reply: make(chan setupAns, 1)}})
	if !strings.Contains(frameOf(m), code) {
		t.Fatalf("code dropped on retry: %q", frameOf(m))
	}
	m = sstep(t, m, setupAskMsg{&setupReq{kind: reqConfirm, prompt: "Change settings?", reply: make(chan setupAns, 1)}})
	if strings.Contains(frameOf(m), code) {
		t.Fatalf("code leaked to a later screen: %q", frameOf(m))
	}
}

func TestModelClearsWarningsOnAnswer(t *testing.T) {
	m := newModel(t)
	m = sstep(t, m, setupNoteMsg{msg: "vault created"})
	m = sstep(t, m, setupNoteMsg{msg: "password must be at least 12 characters", warn: true})
	req := &setupReq{kind: reqConfirm, prompt: "Push?", reply: make(chan setupAns, 1)}
	m = sstep(t, m, setupAskMsg{req})
	m = sstep(t, m, ch('n'))
	<-req.reply
	f := frameOf(m)
	if strings.Contains(f, "at least 12 characters") || !strings.Contains(f, "vault created") {
		t.Fatalf("frame %q", f)
	}
}

func TestModelBoxFitsOptionsInShortWindow(t *testing.T) {
	m := sstep(t, newSetup(SetupOptions{Setup: newSetupUnit(t, &scripted{t: t})}), tea.WindowSizeMsg{Width: 80, Height: 15})
	options := []string{remoteGH, remoteURL, remoteSkip}
	m = sstep(t, m, setupAskMsg{&setupReq{kind: reqSelect, prompt: "Where should the vault sync to", options: options, reply: make(chan setupAns, 1)}})
	f := frameOf(m)
	for _, o := range options {
		if !strings.Contains(f, o) {
			t.Fatalf("option %q hidden in %q", o, f)
		}
	}
	if lines := strings.Split(f, "\n"); len(lines) > 15 {
		t.Fatalf("frame has %d lines", len(lines))
	}
}

func TestModelLongSelectKeepsFocusVisible(t *testing.T) {
	for _, height := range []int{12, 24, 48} {
		m := sstep(t, newModel(t), tea.WindowSizeMsg{Width: 80, Height: height})
		m = sstep(t, m, setupNoteMsg{msg: "vault created"})
		opts := []string{pasteURL}
		for i := range 100 {
			opts = append(opts, fmt.Sprintf("git@github.com:me/repo-%d.git", i))
		}
		m = sstep(t, m, setupAskMsg{&setupReq{kind: reqSelect, prompt: "Pick the repository that holds the vault", options: opts, reply: make(chan setupAns, 1)}})
		for i := range 2 * len(opts) {
			key, idx := down, min(i+1, len(opts)-1)
			if i >= len(opts) {
				key, idx = up, max(2*len(opts)-i-2, 0)
			}
			want := pickPrefix(idx) + opts[idx]
			m = sstep(t, m, key)
			f := frameOf(m)
			if !strings.Contains(f, "▌ "+want) || strings.Count(f, "\n")+1 > height {
				t.Fatalf("height %d step %d want %q focused in %q", height, i, want, f)
			}
		}
	}
}

func TestCtrlCWhileWorkingCancelsTheFlow(t *testing.T) {
	m := newModel(t)
	m = sstep(t, m, ctrl('c'))
	p := m.prompter()
	got := make(chan error, 1)
	go func() {
		_, err := p.Confirm("go on?")
		got <- err
	}()
	select {
	case err := <-got:
		if !errors.Is(err, errSetupCanceled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("next question still asked after ctrl+c")
	}
}
