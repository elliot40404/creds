package update

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func goodTar(t *testing.T) []byte {
	t.Helper()
	return tarGz(t, file("LICENSE", "mit"), file("creds", newBin), file("README.md", "readme"))
}

//go:fix inline
func TestDownloadVerifies(t *testing.T) {
	t.Parallel()
	archive := goodTar(t)
	c, rel := serveRelease(t, fakeRel{files: map[string][]byte{tarAsset: archive}})
	dir := t.TempDir()
	path, err := c.download(t.Context(), rel, tarAsset, dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir || readFile(t, path) != string(archive) {
		t.Fatalf("download %s", path)
	}
}

func TestDownloadRefuses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rel  func([]byte) fakeRel
		want error
	}{
		{"wrong checksum", func(a []byte) fakeRel {
			return fakeRel{files: files(a), sums: new(sumLine(tarAsset, []byte("tampered")))}
		}, ErrChecksum},
		{"missing line", func(a []byte) fakeRel {
			return fakeRel{files: files(a), sums: new(sumLine("creds_0.3.0_darwin_arm64.tar.gz", a))}
		}, ErrChecksum},
		{"duplicate line", func(a []byte) fakeRel {
			return fakeRel{files: files(a), sums: new(sumLine(tarAsset, a) + sumLine(tarAsset, a))}
		}, ErrChecksum},
		{"bad line", func(a []byte) fakeRel {
			return fakeRel{files: files(a), sums: new(sumLine(tarAsset, a) + "zz  other\n")}
		}, ErrChecksum},
		{"single space", func(a []byte) fakeRel {
			return fakeRel{files: files(a), sums: new(strings.Replace(sumLine(tarAsset, a), "  ", " ", 1))}
		}, ErrChecksum},
		{"sums too large", func(a []byte) fakeRel {
			return fakeRel{files: files(a), sums: new(sumLine(tarAsset, a) + strings.Repeat("x", maxSums))}
		}, ErrBadRelease},
		{"no sums", func(a []byte) fakeRel { return fakeRel{files: files(a), noSums: true} }, ErrNoAsset},
		{"no archive", func(a []byte) fakeRel {
			return fakeRel{files: map[string][]byte{"creds_0.3.0_darwin_arm64.tar.gz": a}}
		}, ErrNoAsset},
		{"over cap", func(a []byte) fakeRel {
			return fakeRel{files: files(a), sizes: map[string]int64{tarAsset: maxArchive + 1}}
		}, ErrBadRelease},
		{"longer than size", func(a []byte) fakeRel {
			return fakeRel{files: files(a), sizes: map[string]int64{tarAsset: int64(len(a) - 1)}}
		}, ErrBadRelease},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, rel := serveRelease(t, tc.rel(goodTar(t)))
			dir := t.TempDir()
			if _, err := c.download(t.Context(), rel, tarAsset, dir); !errors.Is(err, tc.want) {
				t.Fatalf("err %v, want %v", err, tc.want)
			}
			if left := leftovers(t, dir); len(left) != 0 {
				t.Fatalf("left %v", left)
			}
		})
	}
}

func files(archive []byte) map[string][]byte {
	return map[string][]byte{tarAsset: archive}
}

func TestDownloadHTTPSOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		scheme string
		path   string
		want   error
	}{
		{"https redirect", "https", "/x/", nil},
		{"http redirect", "http", "/x/", ErrBadRelease},
		{"http asset", "", "", ErrBadRelease},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, rel := serveRelease(t, fakeRel{files: files(goodTar(t))}, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", tc.scheme+"://"+r.Host+"/dl/"+r.PathValue("name"))
				w.WriteHeader(http.StatusFound)
			})
			for i, a := range rel.Assets {
				if a.Name == tarAsset {
					rel.Assets[i].URL = moved(a.URL, tc.path)
				}
			}
			if _, err := c.download(t.Context(), rel, tarAsset, t.TempDir()); !errors.Is(err, tc.want) {
				t.Fatalf("err %v, want %v", err, tc.want)
			}
		})
	}
}

func moved(url, path string) string {
	if path == "" {
		return strings.Replace(url, "https://", "http://", 1)
	}
	return strings.Replace(url, "/dl/", path, 1)
}
