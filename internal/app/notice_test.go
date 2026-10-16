package app

import (
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/gitsync"
)

func TestCheckDue(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		mode    config.UpdateMode
		release string
		checked time.Time
		want    bool
	}{
		{"never checked", config.UpdateHint, "0.2.1", time.Time{}, true},
		{"install mode", config.UpdateInstall, "0.2.1", now.Add(-25 * time.Hour), true},
		{"checked today", config.UpdateHint, "0.2.1", now.Add(-time.Hour), false},
		{"off", config.UpdateOff, "0.2.1", time.Time{}, false},
		{"dev build", config.UpdateHint, "", time.Time{}, false},
		{"odd version", config.UpdateHint, "dev", time.Time{}, false},
	} {
		if got := checkDue(tc.mode, tc.release, tc.checked, now); got != tc.want {
			t.Errorf("%s: checkDue = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNewRelease(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mode    config.UpdateMode
		release string
		latest  string
		want    string
	}{
		{"newer", config.UpdateHint, "0.2.1", "v0.3.0", "0.3.0"},
		{"install mode", config.UpdateInstall, "0.2.1", "v0.3.0", "0.3.0"},
		{"same", config.UpdateHint, "0.2.1", "v0.2.1", ""},
		{"older", config.UpdateHint, "0.3.0", "v0.2.1", ""},
		{"off", config.UpdateOff, "0.2.1", "v0.3.0", ""},
		{"dev build", config.UpdateHint, "", "v0.3.0", ""},
		{"never checked", config.UpdateHint, "0.2.1", "", ""},
	} {
		if got := newRelease(tc.mode, tc.release, tc.latest); got != tc.want {
			t.Errorf("%s: newRelease = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNoticeFromState(t *testing.T) {
	s, _, c := newService(t)
	s.Release = "0.2.1"
	if n := s.Notice(); !n.CheckUpdate || n.Update != "" {
		t.Fatalf("empty state %+v", n)
	}
	if err := gitsync.SaveState(s.Paths.State(), gitsync.State{UpdateChecked: c.t, UpdateLatest: "v0.3.0", ClearError: "boom"}); err != nil {
		t.Fatal(err)
	}
	if n := s.Notice(); n.CheckUpdate || n.Update != "0.3.0" || n.ClearFailure != "boom" {
		t.Fatalf("cached %+v", n)
	}
	if n := s.Notice(); n.ClearFailure != "" {
		t.Fatalf("clear failure shown twice %+v", n)
	}
}
