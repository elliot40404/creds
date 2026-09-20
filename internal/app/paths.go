package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/session"
)

type PathInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Note   string `json:"note,omitzero"`
}

type PathReport struct {
	HomeEnv bool       `json:"home_env"`
	Paths   []PathInfo `json:"paths"`
}

func (s *Service) PathReport() PathReport {
	p := s.Paths
	return PathReport{
		HomeEnv: os.Getenv(config.HomeEnv) != "",
		Paths: []PathInfo{
			info("home", p.Home, ""),
			info("config", p.Config(), ""),
			info("vault", p.Vault(), s.remoteNote()),
			info("session", p.Session(), s.sessionNote()),
			info("state", p.State(), ""),
			info("trust", p.Trust(), ""),
			info("sync.lock", p.SyncLock(), ""),
		},
	}
}

func info(name, path, note string) PathInfo {
	_, err := os.Lstat(path)
	return PathInfo{Name: name, Path: path, Exists: err == nil, Note: note}
}

func (s *Service) remoteNote() string {
	url := s.RemoteURL()
	if url == "" {
		return ""
	}
	return "remote " + safetext.Remote(url)
}

func (s *Service) sessionNote() string {
	left, err := s.sessions().Left()
	if errors.Is(err, session.ErrNoSession) {
		return ""
	}
	if errors.Is(err, session.ErrExpired) {
		return "expired, removed"
	}
	if err != nil {
		return ""
	}
	return left.Round(time.Second).String() + " left"
}

func (s *Service) SessionLeft() time.Duration {
	left, err := s.sessions().Left()
	if err != nil {
		return 0
	}
	return left
}

func (s *Service) RemoteURL() string {
	dir := s.Paths.Vault()
	if _, err := os.Stat(filepath.Join(dir, gitDir)); err != nil {
		return ""
	}
	url, err := gitsync.Open(dir).RemoteURL(context.Background())
	if err != nil {
		return ""
	}
	return url
}

func (s *Service) SetRemote(url string) error {
	ctx := context.Background()
	r, err := s.syncRepo(ctx)
	if err != nil {
		return err
	}
	return r.RemoteSet(ctx, url)
}

func (s *Service) CheckRemote(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitsync.PeekTimeout)
	defer cancel()
	return s.Setup().CheckRemoteEmpty(ctx, url)
}
