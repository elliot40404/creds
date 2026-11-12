package app

import "github.com/elliot40404/creds/internal/sshdir"

type SSHPushOptions struct {
	All         bool
	Overwrite   bool
	KnownOnly   bool
	AskKey      func(rel string) (bool, error)
	AskConflict func(rel string) (bool, error)
}

type SSHPushResult struct {
	Status SSHStatus
	Files  []SSHFileResult
}

func (x SSH) Push(o SSHPushOptions) (SSHPushResult, error) {
	st, err := x.Status()
	if err != nil {
		return SSHPushResult{}, err
	}
	res := SSHPushResult{Status: st, Files: make([]SSHFileResult, len(st.Files))}
	for i, f := range st.Files {
		a, err := o.decide(f)
		if err != nil {
			return SSHPushResult{}, err
		}
		res.Files[i] = SSHFileResult{Rel: f.Rel, Change: f.Change, Action: a}
	}
	if err := x.push(st.Files, res.Files); err != nil {
		return SSHPushResult{}, err
	}
	return res, x.saveState(func(cur sshdir.State) sshdir.State { return nextSSHState(cur, st, res.Files) })
}

func (o SSHPushOptions) decide(f SSHFileStatus) (SSHAction, error) {
	switch f.Change {
	case SSHSame:
		return SSHNoChange, nil
	case SSHLocalNew:
		return o.localNew(f)
	case SSHLocalChanged:
		return SSHPushed, nil
	case SSHConflict:
		if f.Disk == nil {
			return SSHKeptConflict, nil
		}
		return o.conflict(f.Rel)
	case SSHLocalDeleted:
		return SSHKeptDelete, nil
	}
	return SSHNeedsPull, nil
}

func (o SSHPushOptions) localNew(f SSHFileStatus) (SSHAction, error) {
	switch {
	case f.Unused && f.Kind == sshdir.KindPrivateKey:
		return o.key(f)
	case o.KnownOnly:
		return SSHNotAsked, nil
	}
	return SSHPushed, nil
}

func (o SSHPushOptions) key(f SSHFileStatus) (SSHAction, error) {
	switch {
	case o.All:
		return SSHPushed, nil
	case f.Skipped:
		return SSHSkippedKey, nil
	case o.AskKey == nil:
		return SSHNotAsked, nil
	}
	return askPush(o.AskKey, f.Rel, SSHSkippedKey)
}

func (o SSHPushOptions) conflict(rel string) (SSHAction, error) {
	switch {
	case o.Overwrite:
		return SSHPushed, nil
	case o.AskConflict == nil:
		return SSHKeptConflict, nil
	}
	return askPush(o.AskConflict, rel, SSHKeptConflict)
}

func askPush(ask func(string) (bool, error), rel string, no SSHAction) (SSHAction, error) {
	ok, err := ask(rel)
	switch {
	case err != nil:
		return "", err
	case ok:
		return SSHPushed, nil
	}
	return no, nil
}
