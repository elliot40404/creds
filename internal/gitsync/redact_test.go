package gitsync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestErrorHidesURLUserinfo(t *testing.T) {
	t.Parallel()
	e := &Error{
		Args:   []string{"clone", "--", "https://user:TOKEN@host/x.git", `C:\v`},
		Stderr: "fatal: unable to access 'https://user:TOKEN@host/x.git/': boom",
	}
	got := e.Error()
	if strings.Contains(got, "TOKEN") || strings.Contains(got, "user:") {
		t.Fatalf("error = %q", got)
	}
	if !strings.Contains(got, "https://host/x.git") || !strings.Contains(got, `C:\v`) || !strings.Contains(got, "boom") {
		t.Fatalf("error = %q", got)
	}
}

func TestCloneAndPeekErrorsHideToken(t *testing.T) {
	needGit(t)
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	url := strings.Replace(srv.URL, "https://", "https://user:SECRETTOKEN@", 1) + "/x.git"
	_, cloneErr := Clone(context.Background(), url, filepath.Join(t.TempDir(), "v"))
	_, peekErr := Peek(context.Background(), url)
	for name, err := range map[string]error{"clone": cloneErr, "peek": peekErr} {
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		if strings.Contains(err.Error(), "SECRETTOKEN") {
			t.Fatalf("%s: error = %q", name, err)
		}
	}
}
