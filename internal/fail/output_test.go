package fail

import "testing"

func TestOutput(t *testing.T) {
	e := &Error{Msg: "vault is locked", Hint: hintUnlock, Code: Locked}
	if got := Text(e); got != "error: vault is locked\nfix: run creds unlock\n" {
		t.Fatalf("text %q", got)
	}
	if got := Text(&Error{Msg: "boom"}); got != "error: boom\n" {
		t.Fatalf("text no hint %q", got)
	}
	want := `{"error":"vault is locked","hint":"run creds unlock","code":3}`
	if got := string(JSON(e)); got != want {
		t.Fatalf("json %s", got)
	}
}

func TestOutputEscapes(t *testing.T) {
	e := &Error{Msg: "git: \x1b]52;c;eA==\x07", Hint: "run \x1b[2J"}
	if got := Text(e); got != "error: git: \\x1b]52;c;eA==\\x07\nfix: run \\x1b[2J\n" {
		t.Fatalf("text %q", got)
	}
	want := `{"error":"git: \\x1b]52;c;eA==\\x07","hint":"run \\x1b[2J","code":0}`
	if got := string(JSON(e)); got != want {
		t.Fatalf("json %s", got)
	}
}
