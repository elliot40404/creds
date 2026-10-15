package sshdir

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/fsutil"
)

func statePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "ssh.json")
}

func sampleState() State {
	f := File{Rel: "config.d/beam/data_new", Data: []byte("Host beam\n")}
	mod := time.Date(2026, 10, 16, 9, 30, 0, 123456700, time.UTC)
	return State{
		Files:   map[string]FileState{f.Rel: {Sum: f.Sum(), Size: int64(len(f.Data)), ModTime: mod}},
		Skipped: []string{"id_old", "work/id_rsa.pem"},
	}
}

func TestLoadStateMissingIsEmpty(t *testing.T) {
	t.Parallel()
	st, err := LoadState(statePath(t))
	if err != nil || st.Files != nil || st.Skipped != nil {
		t.Fatalf("got %+v %v", st, err)
	}
}

func TestStateRoundTrip(t *testing.T) {
	t.Parallel()
	path := statePath(t)
	want := sampleState()
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	rel := "config.d/beam/data_new"
	g, w := got.Files[rel], want.Files[rel]
	if len(got.Files) != 1 || g.Sum != w.Sum || g.Size != w.Size || !g.ModTime.Equal(w.ModTime) {
		t.Fatalf("files %+v, want %+v", got.Files, want.Files)
	}
	if strings.Join(got.Skipped, ",") != "id_old,work/id_rsa.pem" {
		t.Fatalf("skipped %v", got.Skipped)
	}
	data, err := fsutil.ReadFileLimit(path, maxStateSize)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString(w.Sum[:]))) {
		t.Fatalf("sum not stored as base64: %s", data)
	}
}

func TestSaveStateIsPrivate(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("unix file mode")
	}
	path := statePath(t)
	if err := SaveState(path, sampleState()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != fsutil.FilePerm {
		t.Fatalf("mode %v", perm)
	}
}

func TestSaveStateRefusesBadRel(t *testing.T) {
	t.Parallel()
	for _, st := range []State{
		{Files: map[string]FileState{"../id_rsa": {}}},
		{Skipped: []string{"/etc/passwd"}},
	} {
		path := statePath(t)
		if err := SaveState(path, st); !errors.Is(err, ErrOutside) {
			t.Fatalf("%+v: got %v", st, err)
		}
		if _, err := os.Lstat(path); err == nil {
			t.Fatalf("%+v: file written", st)
		}
	}
}

func TestLoadStateCorrupt(t *testing.T) {
	t.Parallel()
	sum := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cases := map[string]string{
		"not json":     "{files",
		"empty":        "",
		"unknown key":  `{"bogus":1}`,
		"bad base64":   `{"files":{"config":{"sum":"!!"}}}`,
		"short sum":    `{"files":{"config":{"sum":"AAAA"}}}`,
		"dotdot key":   `{"files":{"../config":{"sum":"` + sum + `"}}}`,
		"abs key":      `{"files":{"/etc/config":{"sum":"` + sum + `"}}}`,
		"dotdot skip":  `{"skipped":["config.d/../../id_rsa"]}`,
		"wrong types":  `{"skipped":"id_rsa"}`,
		"bad mod time": `{"files":{"config":{"sum":"` + sum + `","mtime":"yesterday"}}}`,
		"too big":      `{"skipped":["` + strings.Repeat("a", maxStateSize) + `"]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := statePath(t)
			if err := fsutil.WriteFileAtomic(path, []byte(body)); err != nil {
				t.Fatal(err)
			}
			st, err := LoadState(path)
			if !errors.Is(err, ErrStateCorrupt) {
				t.Fatalf("got %+v %v", st, err)
			}
			if !strings.Contains(err.Error(), path) {
				t.Fatalf("path missing from %v", err)
			}
		})
	}
}

func TestUpdateState(t *testing.T) {
	t.Parallel()
	path := statePath(t)
	if _, err := UpdateState(path, func(*State) (bool, error) { return false, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); err == nil {
		t.Fatal("unchanged state written")
	}
	add := func(st *State) (bool, error) {
		st.Skipped = append(st.Skipped, "id_old")
		return true, nil
	}
	if _, err := UpdateState(path, add); err != nil {
		t.Fatal(err)
	}
	got, err := UpdateState(path, add)
	if err != nil || strings.Join(got.Skipped, ",") != "id_old,id_old" {
		t.Fatalf("got %+v %v", got, err)
	}
	boom := errors.New("boom")
	if _, err := UpdateState(path, func(*State) (bool, error) { return true, boom }); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}
