package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/update"
)

func releaseServer(t *testing.T, code int, body string) (update.Checker, *atomic.Int32) {
	t.Helper()
	hits := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return update.Checker{Client: srv.Client(), BaseURL: srv.URL, Repo: update.Repo, Version: "0.2.1"}, hits
}

func loadState(t *testing.T, s *Service) gitsync.State {
	t.Helper()
	st, err := gitsync.LoadState(s.Paths.State())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestCheckUpdateCachesOncePerDay(t *testing.T) {
	s, _, c := newService(t)
	chk, hits := releaseServer(t, http.StatusOK, `{"tag_name":"v0.3.0","html_url":"https://example.test/r"}`)
	if _, err := s.CheckUpdate(t.Context(), chk); err != nil {
		t.Fatal(err)
	}
	st := loadState(t, s)
	if st.UpdateLatest != "v0.3.0" || st.UpdateURL != "https://example.test/r" || !st.UpdateChecked.Equal(c.t) {
		t.Fatalf("state %+v", st)
	}
	c.t = c.t.Add(update.Every - time.Minute)
	if _, err := s.CheckUpdate(t.Context(), chk); err != nil || hits.Load() != 1 {
		t.Fatalf("second check err %v hits %d", err, hits.Load())
	}
	c.t = c.t.Add(time.Minute)
	if _, err := s.CheckUpdate(t.Context(), chk); err != nil || hits.Load() != 2 {
		t.Fatalf("next day err %v hits %d", err, hits.Load())
	}
}

func TestCheckUpdateFailureStillThrottles(t *testing.T) {
	s, _, c := newService(t)
	_, err := gitsync.UpdateState(s.Paths.State(), func(st *gitsync.State) (bool, error) {
		st.UpdateLatest, st.UpdateURL = "v0.2.5", "https://example.test/old"
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	chk, hits := releaseServer(t, http.StatusTooManyRequests, `{}`)
	if _, err := s.CheckUpdate(t.Context(), chk); !errors.Is(err, update.ErrRateLimited) {
		t.Fatalf("got %v", err)
	}
	st := loadState(t, s)
	if !st.UpdateChecked.Equal(c.t) || st.UpdateLatest != "v0.2.5" || st.UpdateURL != "https://example.test/old" {
		t.Fatalf("state %+v", st)
	}
	if _, err := s.CheckUpdate(t.Context(), chk); err != nil || hits.Load() != 1 {
		t.Fatalf("retry err %v hits %d", err, hits.Load())
	}
}

func TestCheckUpdateSkipsDevBuild(t *testing.T) {
	s, _, _ := newService(t)
	chk, hits := releaseServer(t, http.StatusOK, `{"tag_name":"v0.3.0"}`)
	chk.Version = "dev"
	if _, err := s.CheckUpdate(t.Context(), chk); err != nil || hits.Load() != 0 {
		t.Fatalf("err %v hits %d", err, hits.Load())
	}
	if st := loadState(t, s); !st.UpdateChecked.IsZero() {
		t.Fatalf("state %+v", st)
	}
}
