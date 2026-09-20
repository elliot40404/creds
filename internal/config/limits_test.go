package config

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeRejectsAbsurdDurations(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"session.idle":    "[session]\nidle = \"2400h\"\nhard = \"2400h\"\n",
		"session.hard":    "[session]\nhard = \"48h\"\n",
		"clipboard.clear": "[clipboard]\nclear = \"48h\"\n",
		"sync.stale":      "[sync]\nstale = \"9000h\"\n",
	}
	for key, body := range cases {
		_, err := Decode([]byte(body))
		if err == nil || !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "at most") {
			t.Fatalf("%s: %v", key, err)
		}
	}
}

func TestDecodeKeepsMinimum(t *testing.T) {
	t.Parallel()
	_, err := Decode([]byte("[clipboard]\nclear = \"0s\"\n"))
	if err == nil || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("err %v", err)
	}
}

func TestDecodeAllowsTheLimit(t *testing.T) {
	t.Parallel()
	c, err := Decode([]byte("[session]\nidle = \"24h\"\nhard = \"24h\"\n"))
	if err != nil || c.Session.Hard != 24*time.Hour {
		t.Fatalf("c %+v err %v", c, err)
	}
}

func TestWarningsOnlyAboveThreshold(t *testing.T) {
	t.Parallel()
	if w := Warnings(Default()); len(w) != 0 {
		t.Fatalf("default warnings %v", w)
	}
	c := Default()
	c.Session.Idle = 2 * time.Hour
	c.Session.Hard = 20 * time.Hour
	c.Clipboard.Clear = time.Hour
	c.Sync.Stale = 20 * 24 * time.Hour
	w := Warnings(c)
	if len(w) != 4 {
		t.Fatalf("warnings %v", w)
	}
	for _, key := range []string{"session.idle", "session.hard", "clipboard.clear", "sync.stale"} {
		if !strings.Contains(strings.Join(w, "\n"), key) {
			t.Fatalf("%s missing from %v", key, w)
		}
	}
}
