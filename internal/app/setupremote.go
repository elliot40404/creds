package app

import (
	"context"
	"errors"
	"slices"

	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func (u *Setup) ProbeRemote(ctx context.Context, url string) (RemoteState, error) {
	if u.Probe != nil {
		return u.Probe(ctx, url)
	}
	names, err := gitsync.Peek(ctx, url)
	if err != nil {
		return RemoteUnknown, err
	}
	return remoteState(names), nil
}

func remoteState(names []string) RemoteState {
	if len(names) == 0 {
		return RemoteEmpty
	}
	if slices.Contains(names, vaultfiles.MetaFile) && slices.Contains(names, vaultfiles.PasswordFile) {
		return RemoteVault
	}
	return RemoteOther
}

func (u *Setup) CheckRemoteEmpty(ctx context.Context, url string) error {
	state, err := u.ProbeRemote(ctx, url)
	if err != nil {
		return err
	}
	switch state {
	case RemoteVault:
		return ErrRemoteHasVault
	case RemoteEmpty:
		return nil
	case RemoteUnknown, RemoteOther:
	}
	return ErrRemoteNotEmpty
}

func SetupRemoteErr(err error) error {
	if errors.Is(err, gitsync.ErrRepoNotFound) {
		return errRemoteMissing
	}
	return err
}

func RemoteRetryable(err error) bool {
	return errors.Is(err, ErrRemoteHasVault) || errors.Is(err, ErrRemoteNotEmpty) || errors.Is(err, errRemoteMissing)
}

func (u *Setup) ReplaceRemote(url string) error {
	if err := u.Service.SetRemote(url); err != nil {
		return err
	}
	_, err := u.Service.Sync()
	return err
}
