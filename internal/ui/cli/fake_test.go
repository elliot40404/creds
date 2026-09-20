package cli

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/testutil"
)

const (
	mainWord  = "Tr0ub4dor&3x-zebra"
	nextWord  = "Vq8!mZ2#kLp9wR"
	shownCode = "@code"
)

type fake struct {
	passwords []string
	confirms  []bool
	inputs    []string
	selects   []int
	picks     []string
	shown     []string
	prompts   []string
	warned    []string
	noTTY     bool
}

func (f *fake) Warn(msg string) {
	f.warned = append(f.warned, msg)
}

func (f *fake) Password(prompt string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	return testutil.Pop(&f.passwords)
}

func (f *fake) Confirm(prompt string) (bool, error) {
	f.prompts = append(f.prompts, prompt)
	return testutil.Pop(&f.confirms)
}

func (f *fake) Input(prompt, def string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	v, err := testutil.Pop(&f.inputs)
	switch {
	case err != nil:
		return "", err
	case v == shownCode && len(f.shown) > 0:
		return f.shown[len(f.shown)-1], nil
	case v == "":
		return def, nil
	}
	return v, nil
}

func (f *fake) Select(prompt string, options []string) (int, error) {
	f.prompts = append(f.prompts, prompt)
	if len(f.picks) == 0 {
		return testutil.Pop(&f.selects)
	}
	v, _ := testutil.Pop(&f.picks)
	if i := slices.Index(options, v); i >= 0 {
		return i, nil
	}
	return 0, fmt.Errorf("%w: %q not in %v", ErrBadChoice, v, options)
}

func (f *fake) Interactive() bool {
	return !f.noTTY
}

func (f *fake) Show(_, text string) error {
	f.shown = append(f.shown, text)
	return nil
}

func (f *fake) drained() bool {
	return len(f.passwords)+len(f.confirms)+len(f.inputs)+len(f.selects)+len(f.picks) == 0
}

type harness struct {
	t    *testing.T
	home string
	now  time.Time
	clip *fakeClip
}

type result struct {
	code int
	out  string
	err  string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, home: t.TempDir(), now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), clip: &fakeClip{}}
	return h
}

func (h *harness) env(f *fake) (Env, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	return Env{Out: &out, Err: &errb, Prompter: f, Now: func() time.Time { return h.now }, LogN: 10, Clip: h.clip.env(), Home: h.home}, &out, &errb
}

func (h *harness) run(f *fake, args ...string) result {
	h.t.Helper()
	env, out, errb := h.env(f)
	code := Run(env, args)
	return result{code: code, out: out.String(), err: errb.String()}
}

func (h *harness) ok(f *fake, args ...string) result {
	h.t.Helper()
	r := h.run(f, args...)
	if r.code != 0 {
		h.t.Fatalf("%v: code %d stderr %q", args, r.code, r.err)
	}
	if !f.drained() {
		h.t.Fatalf("%v: unused answers %+v", args, f)
	}
	return r
}

func (h *harness) fail(f *fake, args ...string) result {
	h.t.Helper()
	r := h.run(f, args...)
	if r.code == 0 || !strings.HasPrefix(r.err, "error: ") {
		h.t.Fatalf("%v: want failure, got %d %q", args, r.code, r.err)
	}
	return r
}

func (h *harness) init() {
	h.t.Helper()
	h.ok(&fake{passwords: []string{mainWord, mainWord}, inputs: []string{shownCode}}, "init")
}

func (h *harness) expire() {
	h.now = h.now.Add(config.Default().Session.Hard)
}
