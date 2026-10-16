package update

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestInstallSwaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		goos, asset string
		data        func(*testing.T, ...entry) []byte
	}{
		{"linux", tarAsset, tarGz},
		{"windows", zipAsset, zipOf},
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			t.Parallel()
			archive := tc.data(t, file(BinaryName(tc.goos), newBin), file("LICENSE", "mit"))
			c, rel := serveRelease(t, fakeRel{files: map[string][]byte{tc.asset: archive}})
			c.OS = tc.goos
			exe := tempExe(t, "creds.exe")
			if err := c.Install(t.Context(), rel, exe); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, exe); got != newBin {
				t.Fatalf("exe %q", got)
			}
			assertExecutable(t, exe)
			if left := leftovers(t, filepath.Dir(exe)); len(left) != 0 {
				t.Fatalf("left %v", left)
			}
		})
	}
}

func TestInstallRefusedKeepsExe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rel  fakeRel
		want error
	}{
		{"wrong checksum", fakeRel{files: map[string][]byte{tarAsset: []byte("x")}, sums: new(sumLine(tarAsset, []byte("y")))}, ErrChecksum},
		{"no binary", fakeRel{files: map[string][]byte{tarAsset: tarGz(t, file("LICENSE", "mit"))}}, ErrBadRelease},
		{"no asset", fakeRel{files: map[string][]byte{zipAsset: []byte("x")}}, ErrNoAsset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, rel := serveRelease(t, tc.rel)
			exe := tempExe(t, "creds")
			if err := c.Install(t.Context(), rel, exe); !errors.Is(err, tc.want) {
				t.Fatalf("err %v, want %v", err, tc.want)
			}
			if got := readFile(t, exe); got != oldBin {
				t.Fatalf("exe changed to %q", got)
			}
			if left := leftovers(t, filepath.Dir(exe)); len(left) != 0 {
				t.Fatalf("left %v", left)
			}
		})
	}
}

func TestAssetName(t *testing.T) {
	t.Parallel()
	if got := AssetName("v0.3.0", "linux", "arm64"); got != "creds_0.3.0_linux_arm64.tar.gz" {
		t.Fatal(got)
	}
	if got := AssetName("v0.3.0", "windows", "amd64"); got != zipAsset {
		t.Fatal(got)
	}
}
