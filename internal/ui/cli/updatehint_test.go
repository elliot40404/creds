package cli

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/elliot40404/creds/internal/gitsync"
)

const hint030 = "creds 0.3.0 is out, run creds update"

type updateRun struct {
	result
	spawns int
}

func (h *harness) release(version, api string, args ...string) updateRun {
	h.t.Helper()
	env, out, errb := h.env(&fake{})
	var r updateRun
	env.Release = version
	env.Updates = Updates{API: api, Spawn: func() error {
		r.spawns++
		return nil
	}}
	r.code = Run(env, args)
	r.out, r.err = out.String(), errb.String()
	if r.code != 0 {
		h.t.Fatalf("%v: code %d stderr %q", args, r.code, r.err)
	}
	return r
}

func (h *harness) seenRelease(tag string) {
	h.t.Helper()
	st := gitsync.State{UpdateChecked: h.now, UpdateLatest: tag}
	if err := gitsync.SaveState(filepath.Join(h.home, "state.json"), st); err != nil {
		h.t.Fatal(err)
	}
}

func TestUpdateHintWhenNewer(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.seenRelease("v0.3.0")
	r := h.release("0.2.1", "", "config", "get", "session.idle")
	if strings.Count(r.err, hint030) != 1 || strings.Contains(r.out, "0.3.0") || r.spawns != 0 {
		t.Fatalf("stderr %q stdout %q spawns %d", r.err, r.out, r.spawns)
	}
	if r = h.release("0.2.1", "", "version"); strings.Contains(r.err, "is out") {
		t.Fatalf("version hinted %q", r.err)
	}
}

func TestUpdateHintQuiet(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, tag, version, mode string
	}{
		{"same", "v0.2.1", "0.2.1", ""},
		{"older", "v0.2.0", "0.2.1", ""},
		{"off", "v0.3.0", "0.2.1", "off"},
		{"dev", "v0.3.0", "", ""},
	} {
		h := newHarness(t)
		if tc.mode != "" {
			h.ok(&fake{}, "config", "set", "update.mode", tc.mode)
		}
		h.seenRelease(tc.tag)
		if r := h.release(tc.version, "", "config", "get", "session.idle"); r.err != "" || r.spawns != 0 {
			t.Fatalf("%s: stderr %q spawns %d", tc.name, r.err, r.spawns)
		}
	}
}

func TestUpdateCheckSpawnsWhenDue(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if r := h.release("0.2.1", "", "config", "get", "session.idle"); r.spawns != 1 {
		t.Fatalf("spawns %d", r.spawns)
	}
	if r := h.release("", "", "config", "get", "session.idle"); r.spawns != 0 {
		t.Fatalf("dev build spawns %d", r.spawns)
	}
	h.ok(&fake{}, "config", "set", "update.mode", "off")
	if r := h.release("0.2.1", "", "config", "get", "session.idle"); r.spawns != 0 {
		t.Fatalf("off spawns %d", r.spawns)
	}
}

func fakeReleases(t *testing.T, tag string) (string, *atomic.Int32) {
	t.Helper()
	hits := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"tag_name":"` + tag + `","html_url":"https://example.test/r"}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, hits
}

func TestUpdateCheckCachesForTheNextCommand(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	api, hits := fakeReleases(t, "v0.3.0")
	if r := h.release("0.2.1", api, updateCheckName); r.err != "" || r.out != "" {
		t.Fatalf("check printed %q %q", r.out, r.err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits %d", hits.Load())
	}
	r := h.release("0.2.1", api, "config", "get", "session.idle")
	if !strings.Contains(r.err, hint030) || r.spawns != 0 {
		t.Fatalf("stderr %q spawns %d", r.err, r.spawns)
	}
	h.release("0.2.1", api, updateCheckName)
	if hits.Load() != 1 {
		t.Fatalf("checked twice in a day, hits %d", hits.Load())
	}
}

func TestUpdateCheckSkipsOffAndDev(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	api, hits := fakeReleases(t, "v0.3.0")
	h.release("", api, updateCheckName)
	h.ok(&fake{}, "config", "set", "update.mode", "off")
	h.release("0.2.1", api, updateCheckName)
	if hits.Load() != 0 {
		t.Fatalf("hits %d", hits.Load())
	}
}
