package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/config"
)

const (
	mainWord = "Tr0ub4dor&3x-zebra"
	nextWord = "Vq8!mZ2#kLp9wR"
	weakWord = "P@ssw0rd2024!"
)

var errEmpty = errors.New("fake prompter: no answer queued")

type fakePrompter struct {
	passwords []string
	confirms  []bool
	inputs    []string
	shown     []string
	prompts   []string
	warned    []string
}

func (f *fakePrompter) Warn(msg string) {
	f.warned = append(f.warned, msg)
}

func (f *fakePrompter) Password(prompt string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	return pop(&f.passwords)
}

func (f *fakePrompter) Confirm(prompt string) (bool, error) {
	f.prompts = append(f.prompts, prompt)
	return pop(&f.confirms)
}

func (f *fakePrompter) Input(prompt, _ string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	return pop(&f.inputs)
}

func (f *fakePrompter) Select(prompt string, _ []string) (int, error) {
	f.prompts = append(f.prompts, prompt)
	return 0, errEmpty
}

func (f *fakePrompter) Show(_, text string) error {
	f.shown = append(f.shown, text)
	return nil
}

func pop[T any](q *[]T) (T, error) {
	var zero T
	if len(*q) == 0 {
		return zero, errEmpty
	}
	v := (*q)[0]
	*q = (*q)[1:]
	return v, nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newService(t *testing.T) (*Service, *fakePrompter, *clock) {
	t.Helper()
	fp := &fakePrompter{}
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	s := &Service{
		Paths:    config.Paths{Home: t.TempDir()},
		Config:   config.Default(),
		Prompter: fp,
		Now:      c.now,
		LogN:     10,
	}
	return s, fp, c
}

func initVault(t *testing.T) (*Service, *fakePrompter, *clock, string) {
	t.Helper()
	s, fp, c := newService(t)
	fp.passwords = []string{mainWord, mainWord}
	code := ""
	s.Prompter = &codeEcho{fakePrompter: fp, code: &code}
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	s.Prompter = fp
	return s, fp, c, code
}

type codeEcho struct {
	*fakePrompter
	code *string
}

func (c *codeEcho) Show(title, text string) error {
	*c.code = text
	return c.fakePrompter.Show(title, text)
}

func (c *codeEcho) Input(prompt, def string) (string, error) {
	if len(c.inputs) > 0 {
		return c.fakePrompter.Input(prompt, def)
	}
	c.prompts = append(c.prompts, prompt)
	return strings.ToLower(strings.ReplaceAll(*c.code, "-", " ")), nil
}

func expire(s *Service, c *clock) {
	c.t = c.t.Add(s.Config.Session.Hard)
}

func noSecret(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		return
	}
	for _, sec := range secrets {
		if strings.Contains(err.Error(), sec) {
			t.Fatal("error message leaks secret")
		}
	}
}
