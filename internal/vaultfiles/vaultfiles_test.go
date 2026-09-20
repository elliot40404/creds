package vaultfiles

import "testing"

func TestAllowed(t *testing.T) {
	cases := map[string]bool{
		".gitignore":            true,
		"vault.json":            true,
		"identity.pw.age":       true,
		"identity.recovery.age": true,
		"entries/00.enc":        true,
		"entries/0f.enc":        true,
		"entries/ff.enc":        false,
		"entries/.enc":          false,
		"entries/0F.enc":        false,
		"entries/0g.enc":        false,
		"entries/000.enc":       false,
		"entries/0f.enc.bak":    false,
		"entries/a/b.enc":       false,
		"entries/a\\b.enc":      false,
		"entries":               false,
		"x/vault.json":          false,
		"identity.other.age":    false,
		"notes.txt":             false,
	}
	for path, want := range cases {
		if got := Allowed(path); got != want {
			t.Errorf("Allowed(%q) = %v", path, got)
		}
	}
}

func TestFixedIsCopy(t *testing.T) {
	f := Fixed()
	f[0] = "x"
	if Fixed()[0] != ".gitignore" || Allowed("x") {
		t.Fatal("Fixed leaks the shared list")
	}
}

func TestIsBucketOnlyNamesRealSlots(t *testing.T) {
	for name, want := range map[string]bool{"00.enc": true, "0f.enc": true, "10.enc": false, "ff.enc": false, "0g.enc": false, "00.age": false} {
		if got := IsBucket(name); got != want {
			t.Errorf("%s: got %v", name, got)
		}
	}
}
