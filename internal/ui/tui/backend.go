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

func (b ServiceBackend) SSHAddHost(h app.SSHHost) (app.SSHAddHostResult, error) {
	return b.SSH().AddHost(h)
}

func (b ServiceBackend) SSHAddKey(file, name string) (app.SSHAddKeyResult, error) {
	return b.SSH().AddKey(file, name)
}

func (b ServiceBackend) SSHKeygen(k app.SSHKeygen) (app.SSHKeygenResult, error) {
	return b.SSH().Keygen(k)
}
