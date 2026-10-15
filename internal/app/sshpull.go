package app

import "github.com/elliot40404/creds/internal/sshdir"

type SSHPullOptions struct {
	Overwrite   bool
	AskConflict func(f SSHFileStatus) (bool, error)
	AskDelete   func(f SSHFileStatus) (bool, error)
}

type SSHPullResult struct {
	Status SSHStatus
	Files  []SSHFileResult
}

func (s *Service) SSHPull(o SSHPullOptions) (SSHPullResult, error) {
	st, err := s.SSHStatus()
	if err != nil {
		return SSHPullResult{}, err
	}
	res := SSHPullResult{Status: st, Files: make([]SSHFileResult, len(st.Files))}
	for i, f := range st.Files {
		a, err := o.decide(f)
		if err != nil {
			return SSHPullResult{}, err
		}
		res.Files[i] = SSHFileResult{Rel: f.Rel, Change: f.Change, Action: a}
	}
	if err := writeSSH(st, res.Files); err != nil {
		return SSHPullResult{}, err
	}
	return res, s.saveSSHState(func(cur sshdir.State) sshdir.State {
		next := nextSSHState(cur, st, res.Files)
		next.Skipped = cur.Skipped
		return next
	})
}

func (o SSHPullOptions) decide(f SSHFileStatus) (SSHAction, error) {
	switch {
	case f.Change == SSHSame:
		return SSHNoChange, nil
	case f.Change == SSHRemoteNew:
		return SSHCreated, nil
	case f.Change == SSHRemoteChanged:
		return SSHUpdated, nil
	case f.Change == SSHRemoteDeleted, f.Change == SSHConflict && f.Vault == nil:
		return o.remove(f)
	case f.Change == SSHConflict:
		return o.conflict(f)
	}
	return SSHNeedsPush, nil
}

func (o SSHPullOptions) conflict(f SSHFileStatus) (SSHAction, error) {
	yes := SSHUpdated
	if f.Disk == nil {
		yes = SSHCreated
	}
	switch {
	case o.Overwrite:
		return yes, nil
	case o.AskConflict == nil:
		return SSHKeptConflict, nil
	}
	ok, err := o.AskConflict(f)
	return choose(ok, err, yes, SSHKeptConflict)
}

func (o SSHPullOptions) remove(f SSHFileStatus) (SSHAction, error) {
	if o.AskDelete == nil {
		return SSHNotAsked, nil
	}
	ok, err := o.AskDelete(f)
	return choose(ok, err, SSHDeleted, SSHKeptFile)
}

func choose(ok bool, err error, yes, no SSHAction) (SSHAction, error) {
	switch {
	case err != nil:
		return "", err
	case ok:
		return yes, nil
	}
	return no, nil
}
