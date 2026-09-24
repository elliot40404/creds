package config

import (
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.Session.Idle != 15*time.Minute || c.Session.Hard != 4*time.Hour {
		t.Fatalf("session %+v", c.Session)
	}
	if c.Clipboard.Clear != 30*time.Second {
		t.Fatalf("clipboard %+v", c.Clipboard)
	}
	if c.Sync.Stale != 5*time.Minute {
		t.Fatalf("sync %+v", c.Sync)
	}
	if c.Device.MaxAge != 72*time.Hour {
		t.Fatalf("device %+v", c.Device)
	}
	if c.Render.Formats == nil || len(c.Render.Formats) != 0 {
		t.Fatalf("formats %v", c.Render.Formats)
	}
}

func TestDefaultFreshMap(t *testing.T) {
	a := Default()
	a.Render.Formats["x"] = "y"
	if len(Default().Render.Formats) != 0 {
		t.Fatal("formats map shared between defaults")
	}
}
