package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/elliot40404/creds/internal/update"
)

const (
	newExeBody = "new creds binary"
	oldExeBody = "old creds binary"
)

type fakeGitHub struct {
	tag       string
	sums      string
	downloads atomic.Int32
	calls     atomic.Int32
}

func archiveFor(t *testing.T, goos string) []byte {
	t.Helper()
	var buf bytes.Buffer
	name, body := update.BinaryName(goos), []byte(newExeBody)
	if goos == "windows" {
		zw := zip.NewWriter(&buf)
		w, err := zw.Create(name)
		if err == nil {
			_, err = w.Write(body)
		}
		if err = errors.Join(err, zw.Close()); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	if err == nil {
		_, err = tw.Write(body)
	}
	if err = errors.Join(err, tw.Close(), gz.Close()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (g *fakeGitHub) serve(t *testing.T) *httptest.Server {
	t.Helper()
	asset := update.AssetName(g.tag, runtime.GOOS, runtime.GOARCH)
	archive := archiveFor(t, runtime.GOOS)
	sum := sha256.Sum256(archive)
	sums := hex.EncodeToString(sum[:]) + "  " + asset + "\n"
	if g.sums != "" {
		sums = g.sums
	}
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/elliot40404/creds/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		g.calls.Add(1)
		_, _ = fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":"%s/dl/a","size":%d},`+
			`{"name":"checksums.txt","browser_download_url":"%s/dl/sums","size":%d}]}`,
			g.tag, asset, srv.URL, len(archive), srv.URL, len(sums))
	})
	mux.HandleFunc("/dl/a", func(w http.ResponseWriter, _ *http.Request) {
		g.downloads.Add(1)
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/dl/sums", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	srv = httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type updateCase struct {
	release string
	gh      *fakeGitHub
	f       *fake
}

func (h *harness) selfUpdate(c updateCase, args ...string) (result, string) {
	h.t.Helper()
	return h.updateRun(c, append([]string{"update"}, args...)...)
}

func (h *harness) updateRun(c updateCase, args ...string) (result, string) {
	h.t.Helper()
	srv := c.gh.serve(h.t)
	exe := filepath.Join(h.t.TempDir(), "creds.exe")
	if err := os.WriteFile(exe, []byte(oldExeBody), 0o600); err != nil {
		h.t.Fatal(err)
	}
	if c.f == nil {
		c.f = &fake{}
	}
	env, out, errb := h.env(c.f)
	env.Release = c.release
	env.Updates = Updates{API: srv.URL, HTTP: srv.Client(), Exe: func() (string, error) { return exe, nil }}
	r := result{code: Run(env, args)}
	r.out, r.err = out.String(), errb.String()
	data, err := os.ReadFile(filepath.Clean(exe))
	if err != nil {
		h.t.Fatal(err)
	}
	return r, string(data)
}

func TestUpdateInstalls(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args []string
		f    *fake
	}{
		{"yes", []string{"--yes"}, nil},
		{"confirmed", nil, &fake{confirms: []bool{true}}},
	} {
		gh := &fakeGitHub{tag: "v0.3.0"}
		r, exe := newHarness(t).selfUpdate(updateCase{release: "0.2.1", gh: gh, f: tc.f}, tc.args...)
		if r.code != 0 || r.out != "updated creds 0.2.1 -> 0.3.0\n" || exe != newExeBody {
			t.Fatalf("%s: code %d out %q err %q exe %q", tc.name, r.code, r.out, r.err, exe)
		}
	}
}

func TestUpdateNoDownload(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, release, tag, out string
		args                    []string
		f                       *fake
		code                    int
	}{
		{"up to date", "0.3.0", "v0.3.0", "creds is up to date (0.3.0)\n", nil, nil, 0},
		{"older release", "0.3.1", "v0.3.0", "creds is up to date (0.3.1)\n", nil, nil, 0},
		{"check", "0.2.1", "v0.3.0", "creds 0.3.0 is out, this is 0.2.1, run creds update\n", []string{"--check"}, nil, 0},
		{"declined", "0.2.1", "v0.3.0", "", nil, &fake{confirms: []bool{false}}, 1},
	} {
		gh := &fakeGitHub{tag: tc.tag}
		r, exe := newHarness(t).selfUpdate(updateCase{release: tc.release, gh: gh, f: tc.f}, tc.args...)
		if r.code != tc.code || r.out != tc.out || exe != oldExeBody || gh.downloads.Load() != 0 {
			t.Fatalf("%s: code %d out %q err %q exe %q downloads %d", tc.name, r.code, r.out, r.err, exe, gh.downloads.Load())
		}
	}
}

func TestUpdateWrongChecksum(t *testing.T) {
	t.Parallel()
	asset := update.AssetName("v0.3.0", runtime.GOOS, runtime.GOARCH)
	gh := &fakeGitHub{tag: "v0.3.0", sums: strings.Repeat("ab", sha256.Size) + "  " + asset + "\n"}
	r, exe := newHarness(t).selfUpdate(updateCase{release: "0.2.1", gh: gh}, "--yes")
	if r.code == 0 || !strings.Contains(r.err, "checksums") || exe != oldExeBody {
		t.Fatalf("code %d err %q exe %q", r.code, r.err, exe)
	}
}

func TestUpdateDevBuildRefused(t *testing.T) {
	t.Parallel()
	for _, release := range []string{"", "snapshot"} {
		gh := &fakeGitHub{tag: "v0.3.0"}
		r, exe := newHarness(t).selfUpdate(updateCase{release: release, gh: gh}, "--yes")
		want := "go install github.com/elliot40404/creds/cmd/creds@latest or just install"
		if r.code != 2 || !strings.Contains(r.err, want) || exe != oldExeBody || gh.calls.Load() != 0 {
			t.Fatalf("%q: code %d err %q exe %q calls %d", release, r.code, r.err, exe, gh.calls.Load())
		}
	}
}
