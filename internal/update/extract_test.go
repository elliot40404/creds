package update

import (
	"archive/tar"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeArchive(t *testing.T, data []byte) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".creds-download-test")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, dir
}

func TestExtractTakesOnlyTheBinary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		goos string
		data func(*testing.T, ...entry) []byte
		name string
	}{
		{"linux", tarGz, "creds"},
		{"windows", zipOf, "creds.exe"},
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			t.Parallel()
			data := tc.data(t, file("LICENSE", "mit"), file("./"+tc.name, newBin), file("README.md", "readme"))
			archive, dir := writeArchive(t, data)
			bin, err := extract(archive, tc.goos, dir)
			if err != nil {
				t.Fatal(err)
			}
			if readFile(t, bin) != newBin {
				t.Fatalf("binary %q", readFile(t, bin))
			}
			assertExecutable(t, bin)
			if left := leftovers(t, dir); len(left) != 2 {
				t.Fatalf("left %v", left)
			}
		})
	}
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != exePerm {
		t.Fatalf("mode %v", st.Mode())
	}
}

func TestExtractRefuses(t *testing.T) {
	t.Parallel()
	link := entry{name: "creds", link: "/etc/passwd", typ: tar.TypeSymlink}
	big := entry{name: "creds", body: "x", typ: tar.TypeReg, size: maxBinary + 1}
	cases := []struct {
		name    string
		entries []entry
	}{
		{"missing", []entry{file("LICENSE", "mit")}},
		{"traversal", []entry{file("../creds", newBin), file("creds", newBin)}},
		{"absolute", []entry{file("/tmp/creds", newBin), file("creds", newBin)}},
		{"symlink", []entry{link}},
		{"twice", []entry{file("creds", newBin), file("creds", newBin)}},
		{"in a folder", []entry{file("creds_0.3.0/creds", newBin)}},
		{"too large", []entry{big}},
	}
	for _, tc := range cases {
		for _, goos := range []string{"linux", "windows"} {
			t.Run(tc.name+" "+goos, func(t *testing.T) {
				t.Parallel()
				data := tarGz(t, rename(tc.entries, "creds")...)
				if goos == "windows" {
					data = zipOf(t, rename(tc.entries, "creds.exe")...)
				}
				archive, dir := writeArchive(t, data)
				if _, err := extract(archive, goos, dir); !errors.Is(err, ErrBadRelease) {
					t.Fatalf("err %v", err)
				}
				if left := leftovers(t, dir); len(left) != 1 {
					t.Fatalf("left %v", left)
				}
			})
		}
	}
}

func rename(entries []entry, bin string) []entry {
	out := make([]entry, len(entries))
	for i, e := range entries {
		if filepath.Base(e.name) == "creds" {
			e.name = e.name[:len(e.name)-len("creds")] + bin
		}
		out[i] = e
	}
	return out
}

func TestExtractRefusesGarbage(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"linux", "windows"} {
		archive, dir := writeArchive(t, []byte("not an archive"))
		if _, err := extract(archive, goos, dir); !errors.Is(err, ErrBadRelease) {
			t.Fatalf("%s: err %v", goos, err)
		}
	}
}
