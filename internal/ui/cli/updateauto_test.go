package cli

import (
	"crypto/sha256"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/update"
)

const (
	updatedNote = "creds updated to 0.3.0"
	failedNote  = "creds update failed in the background, run creds update"
)

func (h *harness) backgroundUpdate(mode string, gh *fakeGitHub) string {
	h.t.Helper()
	h.ok(&fake{}, "config", "set", "update.mode", mode)
	r, exe := h.updateRun(updateCase{release: "0.2.1", gh: gh}, updateCheckName)
	if r.code != 0 || r.out != "" || r.err != "" {
		h.t.Fatalf("update-check code %d out %q err %q", r.code, r.out, r.err)
	}
	return exe
}

func (h *harness) noteOnce(version, note string) {
	h.t.Helper()
	if r := h.release(version, "", "config", "get", "session.idle"); strings.Count(r.err, note) != 1 {
		h.t.Fatalf("first run stderr %q", r.err)
	}
	if r := h.release(version, "", "config", "get", "session.idle"); strings.Contains(r.err, note) {
		h.t.Fatalf("second run stderr %q", r.err)
	}
}

func TestBackgroundUpdateInstalls(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	gh := &fakeGitHub{tag: "v0.3.0"}
	if exe := h.backgroundUpdate("install", gh); exe != newExeBody || gh.downloads.Load() != 1 {
		t.Fatalf("exe %q downloads %d", exe, gh.downloads.Load())
	}
	h.noteOnce("0.3.0", updatedNote)
}

func TestBackgroundUpdateHintModeOnlyChecks(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	gh := &fakeGitHub{tag: "v0.3.0"}
	if exe := h.backgroundUpdate("hint", gh); exe != oldExeBody || gh.calls.Load() != 1 || gh.downloads.Load() != 0 {
		t.Fatalf("exe %q calls %d downloads %d", exe, gh.calls.Load(), gh.downloads.Load())
	}
	if r := h.release("0.2.1", "", "config", "get", "session.idle"); !strings.Contains(r.err, hint030) {
		t.Fatalf("stderr %q", r.err)
	}
}

func TestBackgroundUpdateSkipsWhenLocked(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	gh := &fakeGitHub{tag: "v0.3.0"}
	var exe string
	err := fsutil.WithFileLock(filepath.Join(h.home, "update.lock"), func() error {
		exe = h.backgroundUpdate("install", gh)
		return nil
	})
	if err != nil || exe != oldExeBody || gh.downloads.Load() != 0 {
		t.Fatalf("err %v exe %q downloads %d", err, exe, gh.downloads.Load())
	}
	st, err := gitsync.LoadState(filepath.Join(h.home, "state.json"))
	if err != nil || st.UpdateError != "" || st.UpdateInstalled != "" {
		t.Fatalf("err %v state %+v", err, st)
	}
}

func TestBackgroundUpdateFailureNotedOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	asset := update.AssetName("v0.3.0", runtime.GOOS, runtime.GOARCH)
	gh := &fakeGitHub{tag: "v0.3.0", sums: strings.Repeat("ab", sha256.Size) + "  " + asset + "\n"}
	if exe := h.backgroundUpdate("install", gh); exe != oldExeBody {
		t.Fatalf("exe %q", exe)
	}
	r := h.release("0.2.1", "", "config", "get", "session.idle")
	if strings.Count(r.err, failedNote) != 1 || !strings.Contains(r.err, hint030) {
		t.Fatalf("stderr %q", r.err)
	}
	if r = h.release("0.2.1", "", "config", "get", "session.idle"); strings.Contains(r.err, failedNote) {
		t.Fatalf("second run stderr %q", r.err)
	}
}
