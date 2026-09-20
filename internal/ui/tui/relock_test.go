package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/vault"
)

func relockModel(t *testing.T, f *fakeBackend) (Model, *[]string) {
	t.Helper()
	var tried []string
	m := start(t, f, Hooks{Prompt: NewPrompt})
	m.opts.Unlock = func(pw string) error {
		tried = append(tried, pw)
		if pw != secretA {
			return crypto.ErrWrongSecret
		}
		f.expired = false
		return nil
	}
	f.expired = true
	return m, &tried
}

func TestRelockPromptsThenRetries(t *testing.T) {
	f := newFake()
	m, tried := relockModel(t, f)
	m = press(m, enter)
	if !strings.Contains(view(m), relockMessage) {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, append(text("bad"), enter)...)
	if !strings.Contains(view(m), "wrong password") {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, append(text(secretA), enter)...)
	noSecrets(t, m)
	if _, ok := m.top().(detailScreen); !ok || !strings.Contains(view(m), "Unlocked") || m.op.retry != nil {
		t.Fatalf("view %q", view(m))
	}
	if strings.Join(*tried, ",") != "bad,"+secretA {
		t.Fatalf("tried %v", *tried)
	}
}

func TestRelockCancel(t *testing.T) {
	f := newFake()
	m, tried := relockModel(t, f)
	m = press(m, enter, esc)
	if _, ok := m.top().(listScreen); !ok || m.quit || m.op.retry != nil || len(*tried) != 0 {
		t.Fatalf("stack %d quit %v", len(m.stack), m.quit)
	}
	if !strings.Contains(view(m), "vault is locked. run creds unlock") {
		t.Fatalf("view %q", view(m))
	}
}

func TestRelockWithoutHook(t *testing.T) {
	f := newFake()
	m := start(t, f, Hooks{})
	f.expired = true
	m = press(m, enter)
	if len(m.stack) != 1 || !strings.Contains(view(m), "session expired. run creds unlock") {
		t.Fatalf("view %q", view(m))
	}
}

func TestRevealAfterExpiryAsksPassword(t *testing.T) {
	f := newFake()
	m := start(t, f, Hooks{Prompt: NewPrompt})
	m.opts.Unlock = func(string) error {
		f.expired = false
		return nil
	}
	m = press(m, down, enter, down)
	f.expired = true
	m = press(m, ch('r'))
	noSecrets(t, m)
	if !strings.Contains(view(m), relockMessage) {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, append(text(secretA), enter)...)
	if _, ok := m.top().(detailScreen); !ok || !strings.Contains(view(m), secretA) {
		t.Fatalf("view %q", view(m))
	}
}

func TestRevealCancelKeepsMasked(t *testing.T) {
	f := newFake()
	m := start(t, f, Hooks{Prompt: NewPrompt})
	m.opts.Unlock = func(string) error { return nil }
	m = press(m, down, enter, down)
	f.expired = true
	m = press(m, ch('r'), esc)
	noSecrets(t, m)
	if _, ok := m.top().(detailScreen); !ok {
		t.Fatalf("view %q", view(m))
	}
}

func TestEditAfterExpiryAsksPassword(t *testing.T) {
	f := newFake()
	m := start(t, f, DefaultHooks())
	m.opts.Unlock = func(string) error {
		f.expired = false
		return nil
	}
	m = press(m, down, enter)
	f.expired = true
	m = press(m, ch('e'))
	if !strings.Contains(view(m), relockMessage) {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, append(text(secretA), enter)...)
	if _, ok := m.top().(formScreen); !ok {
		t.Fatalf("view %q", view(m))
	}
}

func expiredEditForm(t *testing.T, unlock func(string) error) (Model, *fakeBackend) {
	t.Helper()
	f := newFake()
	m := start(t, f, DefaultHooks())
	m.opts.Unlock = unlock
	m = press(m, down, enter, ch('e'), tabKey, tabKey)
	m = press(m, text(secretB+"x")...)
	f.expired = true
	return m, f
}

func TestFormRevealAfterExpiryAsksPassword(t *testing.T) {
	var f *fakeBackend
	m, f := expiredEditForm(t, func(string) error {
		f.expired = false
		return nil
	})
	m = press(m, ctrl('r'))
	if strings.Contains(view(m), secretB+"x") || !strings.Contains(view(m), relockMessage) {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, append(text(secretA), enter)...)
	if _, ok := m.top().(formScreen); !ok || !strings.Contains(view(m), secretB+"x") {
		t.Fatalf("view %q", view(m))
	}
}

func TestFormRevealCancelKeepsMasked(t *testing.T) {
	m, _ := expiredEditForm(t, func(string) error { return nil })
	m = press(m, ctrl('r'), esc)
	if _, ok := m.top().(formScreen); !ok || strings.Contains(view(m), secretB+"x") {
		t.Fatalf("view %q", view(m))
	}
}

func TestFormToggleAfterExpiryAsksPassword(t *testing.T) {
	f := newFake()
	m := start(t, f, DefaultHooks())
	m.opts.Unlock = func(string) error {
		f.expired = false
		return nil
	}
	m = press(m, ch('a'))
	for range slices.Index(app.Types(), vault.TypeGeneric) {
		m = press(m, down)
	}
	m = press(m, enter, ctrl('n'))
	m = press(m, append(text("pin"), enter)...)
	m = press(m, text(secretB)...)
	m = press(m, ctrl('t'))
	f.expired = true
	m = press(m, ctrl('t'))
	if strings.Contains(view(m), secretB) || !strings.Contains(view(m), relockMessage) {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, append(text(secretA), enter)...)
	if _, ok := m.top().(formScreen); !ok || !strings.Contains(view(m), secretB) {
		t.Fatalf("view %q", view(m))
	}
}

func TestHistoryRevealAfterExpiryAsksPassword(t *testing.T) {
	f := newFake()
	f.entries[1] = withHistory()
	m := start(t, f, Hooks{Prompt: NewPrompt})
	m.opts.Unlock = func(string) error {
		f.expired = false
		return nil
	}
	m = press(m, down, enter, ch('h'), enter, down)
	f.expired = true
	m = press(m, ch('r'))
	if strings.Contains(view(m), "oldpw") || !strings.Contains(view(m), relockMessage) {
		t.Fatalf("view %q", view(m))
	}
	m = press(m, append(text(secretA), enter)...)
	if _, ok := m.top().(revisionScreen); !ok || !strings.Contains(view(m), "oldpw") {
		t.Fatalf("view %q", view(m))
	}
}
