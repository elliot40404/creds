package tui

import "github.com/elliot40404/creds/internal/app"

type ServiceBackend struct {
	*app.Service
}

func NewBackend(s *app.Service) ServiceBackend {
	return ServiceBackend{Service: s}
}

func (b ServiceBackend) SSHStatus() (app.SSHStatus, error) {
	return b.SSH().Status()
}

func (b ServiceBackend) SSHPush(o app.SSHPushOptions) (app.SSHPushResult, error) {
	return b.SSH().Push(o)
}

func (b ServiceBackend) SSHPull(o app.SSHPullOptions) (app.SSHPullResult, error) {
	return b.SSH().Pull(o)
}
