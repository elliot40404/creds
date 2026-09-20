package gitsync

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	localRef  = "refs/heads/" + Branch
	remoteRef = "refs/remotes/" + Remote + "/" + Branch
)

var (
	ErrNoRemote     = errors.New("gitsync: no remote configured")
	ErrRemoteExists = errors.New("gitsync: remote already configured")
	ErrBadRemote    = errors.New("gitsync: invalid remote url")
)

type Status struct {
	Remote string
	Ahead  int
	Behind int
}

func (r *Repo) RemoteURL(ctx context.Context) (string, error) {
	out, err := r.git.Run(ctx, "config", "--get", "remote."+Remote+".url")
	if exitCode(err) == 1 {
		return "", nil
	}
	return strings.TrimSpace(string(out)), err
}

func (r *Repo) RemoteAdd(ctx context.Context, url string) error {
	if err := checkURL(url); err != nil {
		return err
	}
	cur, err := r.RemoteURL(ctx)
	if err != nil {
		return err
	}
	if cur != "" {
		return ErrRemoteExists
	}
	_, err = r.git.Run(ctx, "remote", "add", Remote, url)
	return err
}

func (r *Repo) RemoteSet(ctx context.Context, url string) error {
	if err := checkURL(url); err != nil {
		return err
	}
	cur, err := r.RemoteURL(ctx)
	if err != nil {
		return err
	}
	if cur == "" {
		_, err = r.git.Run(ctx, "remote", "add", Remote, url)
		return err
	}
	_, err = r.git.Run(ctx, "remote", "set-url", Remote, url)
	return err
}

func checkURL(url string) error {
	if url == "" || strings.HasPrefix(url, "-") || strings.ContainsAny(url, "\r\n") {
		return ErrBadRemote
	}
	return nil
}

func (r *Repo) RemoteRemove(ctx context.Context) error {
	cur, err := r.RemoteURL(ctx)
	if err != nil {
		return err
	}
	if cur == "" {
		return ErrNoRemote
	}
	_, err = r.git.Run(ctx, "remote", "remove", Remote)
	return err
}

func (r *Repo) Status(ctx context.Context) (Status, error) {
	var st Status
	var err error
	if st.Remote, err = r.RemoteURL(ctx); err != nil {
		return st, err
	}
	st.Ahead, st.Behind, err = r.aheadBehind(ctx)
	return st, err
}

func (r *Repo) Head(ctx context.Context) (string, error) {
	return r.rev(ctx, localRef)
}

func (r *Repo) rev(ctx context.Context, ref string) (string, error) {
	out, err := r.git.Run(ctx, "rev-parse", "--verify", "-q", ref+"^{commit}")
	if exitCode(err) == 1 {
		return "", nil
	}
	return strings.TrimSpace(string(out)), err
}

func (r *Repo) heads(ctx context.Context) (string, string, error) {
	local, err := r.rev(ctx, localRef)
	if err != nil {
		return "", "", err
	}
	remote, err := r.rev(ctx, remoteRef)
	return local, remote, err
}

func (r *Repo) aheadBehind(ctx context.Context) (int, int, error) {
	local, remote, err := r.heads(ctx)
	if err != nil {
		return 0, 0, err
	}
	switch {
	case local == "" && remote == "":
		return 0, 0, nil
	case remote == "":
		n, err := r.count(ctx, local)
		return n, 0, err
	case local == "":
		n, err := r.count(ctx, remote)
		return 0, n, err
	}
	out, err := r.git.Run(ctx, "rev-list", "--max-count", walkLimit, "--left-right", "--count", local+"..."+remote)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("gitsync: bad rev-list output %q", out)
	}
	ahead, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, err
	}
	behind, err := strconv.Atoi(fields[1])
	return ahead, behind, err
}

func (r *Repo) count(ctx context.Context, rev string) (int, error) {
	out, err := r.git.Run(ctx, "rev-list", "--max-count", walkLimit, "--count", rev)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}
