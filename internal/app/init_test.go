package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func TestInitHappyPath(t *testing.T) {
	t.Parallel()
	s, fp, _, code := initVault(t)
	if len(fp.shown) != 1 || fp.shown[0] != code || code == "" {
		t.Fatal("recovery code not shown once")
	}
	for _, name := range []string{vaultfiles.PasswordFile, vaultfiles.RecoveryFile, vaultfiles.MetaFile, "entries/00.enc", "entries/0f.enc"} {
		if _, err := os.Stat(s.vaultFile(name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	data, err := os.ReadFile(s.vaultFile(vaultfiles.RecoveryFile))
	if err != nil || !crypto.IsAgeFile(data) {
		t.Fatal("recovery file not age")
	}
	if err := s.Unlock(); err != nil {
		t.Fatalf("unlock after init: %v", err)
	}
	if slices.Contains(fp.prompts, "Master password") {
		t.Fatal("init did not leave a session")
	}
}

func TestInitRefusesExisting(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.prompts = nil
	if err := s.Init(); !errors.Is(err, ErrVaultExists) {
		t.Fatalf("err = %v", err)
	}
	if len(fp.prompts) != 0 {
		t.Fatal("prompted before existence check")
	}
}

func TestInitRejects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		passwords []string
		confirms  []bool
		inputs    []string
		want      error
	}{
		{"short", []string{"Xy7!kPq2", "a", "b"}, nil, nil, ErrShortPassword},
		{"weak", []string{weakWord, weakWord, weakWord}, nil, nil, ErrWeakPassword},
		{"mismatch", []string{mainWord, nextWord, mainWord, nextWord, mainWord, nextWord}, nil, nil, ErrPasswordMismatch},
		{"recovery mistyped", []string{mainWord, mainWord}, nil, []string{"AAAAA-BBBBB", "x", "y"}, ErrRecoveryMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, fp, _ := newService(t)
			fp.passwords, fp.confirms, fp.inputs = tt.passwords, tt.confirms, tt.inputs
			err := s.Init()
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v", err)
			}
			noSecret(t, err, mainWord, nextWord, weakWord)
			if _, err := os.Stat(s.vaultFile(vaultfiles.MetaFile)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("vault written")
			}
			if _, err := os.Stat(filepath.Join(s.Paths.Vault(), vaultfiles.PasswordFile)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("identity written")
			}
		})
	}
}

func TestInitWeakNeverAccepted(t *testing.T) {
	t.Parallel()
	s, fp, _ := newService(t)
	fp.passwords = []string{weakWord, weakWord, weakWord}
	code := ""
	s.Prompter = &codeEcho{fakePrompter: fp, code: &code}
	if err := s.Init(); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("err = %v", err)
	}
	for _, p := range fp.prompts {
		if strings.Contains(strings.ToLower(p), "weak") {
			t.Fatalf("weak password prompt offered: %q", p)
		}
	}
}

func TestInitRetries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		passwords []string
		confirms  []bool
		inputs    int
		warn      string
	}{
		{"short", []string{"Xy7!kPq2", mainWord, mainWord}, nil, 0, "at least"},
		{"weak", []string{weakWord, mainWord, mainWord}, nil, 0, "password is too weak"},
		{"mismatch", []string{mainWord, nextWord, nextWord, nextWord}, nil, 0, "do not match"},
		{"recovery typo", []string{mainWord, mainWord}, nil, 2, "recovery code does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, fp, _ := newService(t)
			fp.passwords, fp.confirms = tt.passwords, tt.confirms
			if tt.inputs > 0 {
				fp.inputs = []string{"AAAAA-BBBBB"}
			}
			code := ""
			s.Prompter = &codeEcho{fakePrompter: fp, code: &code}
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			if len(fp.shown) != 1 || len(fp.passwords) != 0 {
				t.Fatalf("shown %d left %d", len(fp.shown), len(fp.passwords))
			}
			if len(fp.warned) != 1 || !strings.Contains(fp.warned[0], tt.warn) || !strings.Contains(fp.warned[0], "try again") {
				t.Fatalf("warned %q", fp.warned)
			}
			noSecret(t, errors.New(strings.Join(fp.warned, " ")), mainWord, nextWord)
		})
	}
}

func TestInitRefusesLeftovers(t *testing.T) {
	t.Parallel()
	for _, name := range []string{".git", vaultfiles.RecoveryFile, "entries/03.enc"} {
		s, fp, _ := newService(t)
		path := s.vaultFile(name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := s.Init(); !errors.Is(err, ErrVaultIncomplete) {
			t.Fatalf("%s: err = %v", name, err)
		}
		if len(fp.prompts) != 0 {
			t.Fatalf("%s: prompted %v", name, fp.prompts)
		}
	}
}

func TestMissingMetaIsIncomplete(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	if err := os.Remove(s.vaultFile(vaultfiles.MetaFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(); !errors.Is(err, ErrVaultIncomplete) {
		t.Fatalf("list err = %v", err)
	}
	fp.passwords = []string{mainWord, mainWord}
	if err := s.Init(); !errors.Is(err, ErrVaultIncomplete) {
		t.Fatalf("init err = %v", err)
	}
	if _, err := os.Lstat(s.vaultFile("entries/00.enc")); err != nil {
		t.Fatal(err)
	}
}
