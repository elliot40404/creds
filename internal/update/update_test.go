package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const okBody = `{"tag_name":"v0.3.0","html_url":"https://example.test/r/v0.3.0","draft":false,"prerelease":false,"id":7,"body":"notes",` +
	`"assets":[{"name":"creds_0.3.0_linux_amd64.tar.gz","browser_download_url":"https://example.test/a.tar.gz","size":42,"label":null}]}`

func serve(t *testing.T, h http.HandlerFunc) Checker {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return Checker{Client: srv.Client(), BaseURL: srv.URL, Repo: Repo, Version: "0.2.1"}
}

func reply(code int, body string, headers ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for i := 0; i+1 < len(headers); i += 2 {
			w.Header().Set(headers[i], headers[i+1])
		}
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}
}

func TestLatestSendsRequest(t *testing.T) {
	var got *http.Request
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		reply(http.StatusOK, okBody)(w, r)
	})
	rel, err := c.Latest(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodGet || got.URL.Path != "/repos/elliot40404/creds/releases/latest" {
		t.Fatalf("request %s %s", got.Method, got.URL.Path)
	}
	if got.Header.Get("Accept") != mimeJSON || got.Header.Get("User-Agent") != "creds/0.2.1" || got.Header.Get("Authorization") != "" {
		t.Fatalf("headers %v", got.Header)
	}
	if rel.Tag != "v0.3.0" || rel.URL != "https://example.test/r/v0.3.0" || len(rel.Assets) != 1 {
		t.Fatalf("release %+v", rel)
	}
	if a := rel.Assets[0]; a.Name != "creds_0.3.0_linux_amd64.tar.gz" || a.URL != "https://example.test/a.tar.gz" || a.Size != 42 {
		t.Fatalf("asset %+v", a)
	}
}

func TestLatestErrors(t *testing.T) {
	big := `{"tag_name":"v0.3.0","body":"` + strings.Repeat("x", maxBody) + `"}`
	cases := []struct {
		name string
		h    http.HandlerFunc
		want error
	}{
		{"bad json", reply(http.StatusOK, `{"tag_name":`), ErrBadRelease},
		{"wrong type", reply(http.StatusOK, `{"tag_name":3}`), ErrBadRelease},
		{"bad tag", reply(http.StatusOK, `{"tag_name":"latest"}`), ErrBadRelease},
		{"draft", reply(http.StatusOK, `{"tag_name":"v0.3.0","draft":true}`), ErrBadRelease},
		{"prerelease", reply(http.StatusOK, `{"tag_name":"v0.3.0-rc1","prerelease":true}`), ErrBadRelease},
		{"oversize", reply(http.StatusOK, big), ErrBadRelease},
		{"rate limit 403", reply(http.StatusForbidden, `{}`, "X-RateLimit-Remaining", "0"), ErrRateLimited},
		{"rate limit 429", reply(http.StatusTooManyRequests, `{}`), ErrRateLimited},
		{"forbidden", reply(http.StatusForbidden, `{}`, "X-RateLimit-Remaining", "12"), ErrBadRelease},
		{"not found", reply(http.StatusNotFound, `{}`), ErrBadRelease},
		{"server error", reply(http.StatusInternalServerError, `oops`), ErrBadRelease},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := serve(t, tc.h).Latest(t.Context())
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLatestOffline(t *testing.T) {
	srv := httptest.NewServer(reply(http.StatusOK, okBody))
	c := Checker{Client: srv.Client(), BaseURL: srv.URL, Repo: Repo, Version: "0.2.1"}
	srv.Close()
	if _, err := c.Latest(t.Context()); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("got %v", err)
	}
}

func TestLatestTimeout(t *testing.T) {
	release := make(chan struct{})
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer close(release)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Latest(ctx)
	if !errors.Is(err, ErrUnreachable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}

func TestNewDefaults(t *testing.T) {
	c := New("0.2.1")
	if c.BaseURL != API || c.Repo != Repo || c.Version != "0.2.1" || c.Client == nil || c.Client.Timeout != Timeout {
		t.Fatalf("checker %+v", c)
	}
}
