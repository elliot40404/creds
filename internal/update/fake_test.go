package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	newBin   = "new creds binary"
	oldBin   = "old creds binary"
	tarAsset = "creds_0.3.0_linux_amd64.tar.gz"
	zipAsset = "creds_0.3.0_windows_amd64.zip"
)

type entry struct {
	name, body, link string
	typ              byte
	size             int64
}

func file(name, body string) entry {
	return entry{name: name, body: body, typ: tar.TypeReg}
}

func tarGz(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	for _, e := range entries {
		size := cmpSize(e)
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Linkname: e.link, Typeflag: e.typ, Size: size, Mode: 0o755}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil && e.size == 0 {
			t.Fatal(err)
		}
	}
	if !slices.ContainsFunc(entries, func(e entry) bool { return e.size > 0 }) {
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	_, _ = gz.Write(raw.Bytes())
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func cmpSize(e entry) int64 {
	if e.size > 0 {
		return e.size
	}
	if e.typ == tar.TypeReg {
		return int64(len(e.body))
	}
	return 0
}

func zipOf(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Store}
		h.SetMode(0o755)
		if e.typ == tar.TypeSymlink {
			h.SetMode(os.ModeSymlink | 0o777)
		}
		w, err := zipEntry(zw, h, e)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(e.body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func zipEntry(zw *zip.Writer, h *zip.FileHeader, e entry) (interface{ Write([]byte) (int, error) }, error) {
	if e.size == 0 {
		return zw.CreateHeader(h)
	}
	h.UncompressedSize64, h.CompressedSize64 = maxBinary+1, 1
	return zw.CreateRaw(h)
}

func sumLine(name string, data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:]) + "  " + name + "\n"
}

type fakeRel struct {
	files  map[string][]byte
	sizes  map[string]int64
	sums   *string
	noSums bool
}

func serveRelease(t *testing.T, f fakeRel, h ...http.HandlerFunc) (Checker, Release) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/{name}", func(w http.ResponseWriter, r *http.Request) {
		data, ok := f.files[r.PathValue("name")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
	for _, fn := range h {
		mux.HandleFunc("/x/{name}", fn)
	}
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	rel := Release{Tag: "v0.3.0", Assets: assets(srv.URL, f)}
	return Checker{Download: srv.Client(), Version: "0.2.1", OS: "linux", Arch: "amd64"}, rel
}

func assets(base string, f fakeRel) []Asset {
	sums := ""
	var out []Asset
	for name, data := range f.files {
		sums += sumLine(name, data)
		size := int64(len(data))
		if s, ok := f.sizes[name]; ok {
			size = s
		}
		out = append(out, Asset{Name: name, URL: base + "/dl/" + name, Size: size})
	}
	if f.sums != nil {
		sums = *f.sums
	}
	if !f.noSums {
		f.files[SumsName] = []byte(sums)
		out = append(out, Asset{Name: SumsName, URL: base + "/dl/" + SumsName, Size: int64(len(sums))})
	}
	return out
}

func tempExe(t *testing.T, name string) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(exe, []byte(oldBin), 0o600); err != nil {
		t.Fatal(err)
	}
	return exe
}

func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".creds-") {
			out = append(out, e.Name())
		}
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
