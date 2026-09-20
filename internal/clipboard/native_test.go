package clipboard

import (
	"os"
	"testing"
)

func TestSystemRoundtrip(t *testing.T) {
	if os.Getenv("CREDS_CLIPBOARD_TEST") == "" {
		t.Skip("set CREDS_CLIPBOARD_TEST=1 to use the real clipboard")
	}
	var s System
	saved, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Write(saved) })
	const value = "creds clipboard test ünï"
	if err := s.Write(value); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read()
	if err != nil || got != value {
		t.Fatalf("read %q, %v", got, err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Read(); got != "" {
		t.Fatalf("after clear %q", got)
	}
}
