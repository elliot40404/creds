package app

import "github.com/elliot40404/creds/internal/sshdir"

type SSHAction string

const (
	SSHPushed     SSHAction = "pushed"
	SSHNoChange   SSHAction = "same"
	SSHSkippedKey SSHAction = "skipped"
	SSHNotAsked   SSHAction = "not asked"
	SSHKeptVault  SSHAction = "conflict"
	SSHKeptDelete SSHAction = "deleted here"
	SSHNeedsPull  SSHAction = "pull"
	SSHFailed     SSHAction = "failed"
)

type SSHPushOptions struct {
	All         bool
	Overwrite   bool
	AskKey      func(rel string) (bool, error)
	AskConflict func(rel string) (bool, error)
}

type SSHPushFile struct {
	Rel    string
	Change SSHChange
	Action SSHAction
	Err    error
}

type SSHPushResult struct {
	Status SSHStatus
	Files  []SSHPushFile
}

func (s *Service) SSHPush(o SSHPushOptions) (SSHPushResult, error) {
	st, err := s.SSHStatus()
	if err != nil {
		return SSHPushResult{}, err
	}
	res := SSHPushResult{Status: st, Files: make([]SSHPushFile, len(st.Files))}
	for i, f := range st.Files {
		a, err := o.decide(f)
		if err != nil {
			return SSHPushResult{}, err
		}
		res.Files[i] = SSHPushFile{Rel: f.Rel, Change: f.Change, Action: a}
	}
	if err := s.pushSSH(st.Files, res.Files); err != nil {
		return SSHPushResult{}, err
	}
	return res, s.saveSSHState(st, res.Files)
}

func (o SSHPushOptions) decide(f SSHFileStatus) (SSHAction, error) {
	switch f.Change {
	case SSHSame:
		return SSHNoChange, nil
	case SSHLocalNew:
		if f.Unused && f.Kind == sshdir.KindPrivateKey {
			return o.key(f)
		}
		return SSHPushed, nil
	case SSHLocalChanged:
		return SSHPushed, nil
	case SSHConflict:
		return o.conflict(f.Rel)
	case SSHLocalDeleted:
		return SSHKeptDelete, nil
	}
	return SSHNeedsPull, nil
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
		return SSHKeptVault, nil
	}
	return askPush(o.AskConflict, rel, SSHKeptVault)
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
