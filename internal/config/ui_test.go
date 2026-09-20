package config

import (
	"errors"
	"strconv"
	"testing"
)

func TestDefaultUI(t *testing.T) {
	ui := Default().UI
	if ui.Mode != ModeFullscreen || !ui.AltScreen || ui.Height != DefaultHeight {
		t.Fatalf("default ui %#v", ui)
	}
}

func TestUIHeightBounds(t *testing.T) {
	for _, h := range []int{0, MinHeight - 1, MaxHeight + 1} {
		c := Default()
		if _, err := Set(c, "ui.height", strconv.Itoa(h)); err == nil {
			t.Fatalf("height %d accepted", h)
		}
	}
	for _, h := range []int{MinHeight, DefaultHeight, MaxHeight} {
		c, err := Set(Default(), "ui.height", strconv.Itoa(h))
		if err != nil || c.UI.Height != h {
			t.Fatalf("height %d: %v", h, err)
		}
	}
}

func TestUIModeRejectsJunk(t *testing.T) {
	if _, err := Set(Default(), "ui.mode", "tiny"); !errors.Is(err, ErrUnknownMode) {
		t.Fatalf("err %v", err)
	}
	c, err := Set(Default(), "ui.mode", "inline")
	if err != nil || c.UI.Mode != ModeInline {
		t.Fatalf("inline: %v %q", err, c.UI.Mode)
	}
}

func TestUIAltScreenToggles(t *testing.T) {
	c, err := Set(Default(), "ui.altscreen", "false")
	if err != nil || c.UI.AltScreen {
		t.Fatalf("altscreen: %v %v", err, c.UI.AltScreen)
	}
	if _, err := Set(Default(), "ui.altscreen", "maybe"); !errors.Is(err, ErrNotBool) {
		t.Fatalf("err %v", err)
	}
}

func TestUIRoundTrips(t *testing.T) {
	c := Default()
	c.UI = UI{Mode: ModeInline, AltScreen: false, Height: 22}
	data, err := Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.UI != c.UI {
		t.Fatalf("ui %#v, want %#v", got.UI, c.UI)
	}
}

func TestMissingUISectionKeepsDefaults(t *testing.T) {
	got, err := Decode([]byte("[session]\nidle = \"5m\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.UI != Default().UI {
		t.Fatalf("ui %#v", got.UI)
	}
}
