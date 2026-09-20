package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/crypto"
)

func TestPromptSubmitMasked(t *testing.T) {
	s := NewPrompt("Session expired, unlock to continue")
	s, _ = keys(s, text(secretA)...)
	v := formView(s)
	if strings.Contains(v, secretA) || !strings.Contains(v, "Session expired") || !strings.Contains(v, strings.Repeat("*", len(secretA))) {
		t.Fatalf("view %q", v)
	}
	s, msg := keys(s, enter)
	if res, ok := msg.(PasswordResult); !ok || res.Password != secretA || res.Canceled {
		t.Fatalf("msg %#v", msg)
	}
	if !strings.Contains(formView(s), "checking") {
		t.Fatalf("view %q", formView(s))
	}
	if _, msg = keys(s, enter); msg != nil {
		t.Fatalf("second enter while waiting sent %#v", msg)
	}
}

func TestPromptWrongPassword(t *testing.T) {
	s, _ := keys(NewPrompt("unlock"), append(text("bad"), enter)...)
	s, _ = s.Update(PasswordError{Err: fmt.Errorf("unlock: %w", crypto.ErrWrongSecret)})
	if v := formView(s); !strings.Contains(v, "wrong password. try again") || strings.Contains(v, "bad") {
		t.Fatalf("view %q", v)
	}
	s, _ = keys(s, text("x")...)
	if strings.Contains(formView(s), "wrong password") {
		t.Fatal("error should clear on typing")
	}
	s, _ = s.Update(PasswordError{Err: errors.New("disk gone\ndetails")})
	if v := formView(s); !strings.Contains(v, "unlock failed: disk gone") || strings.Contains(v, "details") {
		t.Fatalf("view %q", v)
	}
	s, _ = s.Update(tea.PasteMsg{Content: "yz"})
	_, msg := keys(s, back, enter)
	if res, ok := msg.(PasswordResult); !ok || res.Password != "xy" {
		t.Fatalf("msg %#v", msg)
	}
}

func TestPromptEmptyAndCancel(t *testing.T) {
	s, msg := keys(NewPrompt("unlock"), enter)
	if msg != nil || !strings.Contains(formView(s), "password is empty") {
		t.Fatalf("msg %#v view %q", msg, formView(s))
	}
	s, _ = keys(s, text("abc")...)
	s, msg = keys(s, esc)
	if res, ok := msg.(PasswordResult); !ok || !res.Canceled || res.Password != "" {
		t.Fatalf("msg %#v", msg)
	}
	p := s.(promptScreen)
	if !p.input.empty() || !p.typing() || len(p.keys()) == 0 {
		t.Fatal("cancel should clear input")
	}
}
