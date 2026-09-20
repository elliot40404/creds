package cli

import (
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/testutil"
)

func TestInitTwiceFails(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	r := h.fail(&fake{}, "init")
	if !strings.Contains(r.err, "already exists") {
		t.Fatalf("err = %q", r.err)
	}
}

func TestInitOutputGoesToStderr(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := &fake{passwords: []string{mainWord, mainWord}, inputs: []string{shownCode}}
	r := h.ok(f, "init")
	if r.out != "" {
		t.Fatalf("stdout = %q", r.out)
	}
	if !strings.Contains(r.err, "vault created, next run creds add -i") || strings.Contains(r.err, mainWord) {
		t.Fatalf("stderr = %q", r.err)
	}
}

func TestLockUnlock(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "unlock")
	h.ok(&fake{}, "lock")
	h.fail(&fake{passwords: []string{nextWord}}, "unlock")
	h.ok(&fake{passwords: []string{mainWord}}, "unlock")
	h.ok(&fake{}, "unlock")
}

func TestUnlockWithoutVault(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.fail(&fake{}, "unlock")
	if !strings.Contains(r.err, "not initialized") {
		t.Fatalf("err = %q", r.err)
	}
}

func TestPasswd(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{passwords: []string{mainWord, nextWord, nextWord}}, "passwd")
	h.ok(&fake{}, "lock")
	h.fail(&fake{passwords: []string{mainWord}}, "unlock")
	h.ok(&fake{passwords: []string{nextWord}}, "unlock")
}

func TestRecover(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	f := &fake{passwords: []string{mainWord, mainWord}, inputs: []string{shownCode}}
	h.ok(f, "init")
	code := f.shown[0]
	h.ok(&fake{}, "lock")
	h.ok(&fake{passwords: []string{code, nextWord, nextWord}}, "recover")
	h.ok(&fake{}, "lock")
	h.ok(&fake{passwords: []string{nextWord}}, "unlock")
}

func TestRecoverWrongCode(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	code, err := crypto.NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	r := h.fail(&fake{passwords: []string{code}}, "recover")
	if strings.Contains(r.err, code) {
		t.Fatal("error leaks code")
	}
}

func TestSessionExpiryPrompts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.expire()
	f := &fake{}
	h.fail(f, "unlock")
	if len(f.prompts) != 1 {
		t.Fatalf("prompts = %v", f.prompts)
	}
}

func TestPromptFailureExits(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	h.ok(&fake{}, "lock")
	r := h.fail(&fake{}, "unlock")
	if !strings.Contains(r.err, testutil.ErrNoAnswer.Error()) {
		t.Fatalf("err = %q", r.err)
	}
}

func TestExtraArgsRejected(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, c := range []string{"init", "lock", "unlock", "passwd", "recover"} {
		r := h.fail(&fake{}, c, "x")
		if r.out != "" || !strings.Contains(r.err, "unknown command") && !strings.Contains(r.err, "accepts 0 arg") {
			t.Fatalf("%s out = %q", c, r.out)
		}
	}
}
