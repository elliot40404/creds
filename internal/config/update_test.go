package config

import (
	"errors"
	"strings"
	"testing"
)

func TestUpdateModeDefaultsToHintUnwritten(t *testing.T) {
	t.Parallel()
	if v, err := Get(Default(), "update.mode"); err != nil || v != "hint" {
		t.Fatalf("get %q %v", v, err)
	}
	data, err := Encode(Default())
	if err != nil || strings.Contains(string(data), "[update]") {
		t.Fatalf("default written %q %v", data, err)
	}
}

func TestUpdateModeSetAndRoundtrip(t *testing.T) {
	t.Parallel()
	off, err := Set(Default(), "update.mode", " off ")
	if err != nil || off.Update.Current() != UpdateOff || Default().Update.Mode != "" {
		t.Fatalf("set %+v %v", off.Update, err)
	}
	data, err := Encode(off)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(data)
	if err != nil || back.Update.Current() != UpdateOff {
		t.Fatalf("roundtrip %q %+v %v", data, back.Update, err)
	}
}

func TestUpdateModeRejectsUnknown(t *testing.T) {
	t.Parallel()
	if _, err := Set(Default(), "update.mode", "always"); !errors.Is(err, ErrUnknownUpdate) {
		t.Fatalf("set %v", err)
	}
	if _, err := Decode([]byte("[update]\nmode = \"always\"\n")); !errors.Is(err, ErrUnknownUpdate) {
		t.Fatalf("decode %v", err)
	}
}

func TestUpdateModeCycles(t *testing.T) {
	t.Parallel()
	if v, err := Next("update.mode", "hint", 1); err != nil || v != "install" {
		t.Fatalf("next %q %v", v, err)
	}
}
