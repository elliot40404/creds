package gitsync

import (
	"context"
	"errors"
	"maps"
	"strings"
)

type SyncResult int

const (
	UpToDate SyncResult = iota
	Pushed
	FastForwarded
	Merged
	NeedsUnlock
)

var (
	ErrDiverged     = errors.New("gitsync: local and remote have diverged")
	ErrPushRejected = errors.New("gitsync: push rejected by remote")
)

func (r *Repo) Sync(ctx context.Context, opts SyncOptions) (SyncResult, error) {
	url, err := r.RemoteURL(ctx)
	if err != nil {
		return UpToDate, err
	}
	if url == "" {
		return UpToDate, ErrNoRemote
	}
	res, err := r.syncOnce(ctx, opts)
	if errors.Is(err, ErrPushRejected) {
		res, err = r.syncOnce(ctx, opts)
	}
	if err != nil {
		return res, err
	}
	return res, r.advanceAnchor(ctx, opts, res)
}

func (r *Repo) syncOnce(ctx context.Context, opts SyncOptions) (SyncResult, error) {
	if err := r.fetch(ctx); err != nil {
		return UpToDate, err
	}
	local, remote, err := r.heads(ctx)
	if err != nil {
		return UpToDate, err
	}
	switch {
	case local == remote:
		return UpToDate, r.verifyAdopt(ctx, opts, remote)
	case local == "":
		return r.fastForward(ctx, opts, local, remote)
	case remote == "":
		return r.push(ctx)
	}
	ahead, err := r.isAncestor(ctx, remote, local)
	if err != nil {
		return UpToDate, err
	}
	if ahead {
		return r.push(ctx)
	}
	behind, err := r.isAncestor(ctx, local, remote)
	if err != nil {
		return UpToDate, err
	}
	if behind {
		return r.fastForward(ctx, opts, local, remote)
	}
	if opts.Identity == nil {
		return UpToDate, ErrDiverged
	}
	return r.merge(ctx, opts, local, remote)
}

func (r *Repo) fetch(ctx context.Context) error {
	r.runHook("fetch")
	_, err := r.git.Run(ctx, "fetch", "-q", "--no-tags", "--no-write-fetch-head", Remote, "+"+localRef+":"+remoteRef)
	if err == nil || !missingRemoteBranch(err) {
		return err
	}
	_, err = r.git.Run(ctx, "update-ref", "-d", remoteRef)
	return err
}

func missingRemoteBranch(err error) bool {
	gerr, ok := errors.AsType[*Error](err)
	return ok && strings.Contains(gerr.Stderr, "couldn't find remote ref")
}

func (r *Repo) isAncestor(ctx context.Context, a, b string) (bool, error) {
	_, err := r.git.Run(ctx, "merge-base", "--is-ancestor", a, b)
	if exitCode(err) == 1 {
		return false, nil
	}
	return err == nil, err
}

func (r *Repo) fastForward(ctx context.Context, opts SyncOptions, local, remote string) (SyncResult, error) {
	id := opts.Identity
	lt, it, err := r.revTrees(ctx, local, remote)
	if err != nil {
		return UpToDate, err
	}
	if id == nil && !maps.Equal(lt, it) {
		return NeedsUnlock, nil
	}
	if len(changedBuckets(lt, it)) > 0 {
		if err := r.openBuckets(ctx, id, changedBuckets(tree{}, it)); err != nil {
			return UpToDate, err
		}
	}
	if err := r.verifyAdopt(ctx, opts, remote); err != nil {
		return UpToDate, err
	}
	if _, err := r.git.Run(ctx, "merge", "-q", "--ff-only", remote); err != nil {
		return UpToDate, err
	}
	return FastForwarded, nil
}

func (r *Repo) push(ctx context.Context) (SyncResult, error) {
	r.runHook("push")
	out, err := r.git.Run(ctx, "push", "--porcelain", "--no-verify", Remote, localRef+":"+localRef)
	if err == nil {
		return Pushed, nil
	}
	if rejected(string(out)) {
		return UpToDate, ErrPushRejected
	}
	return UpToDate, err
}

func rejected(porcelain string) bool {
	for line := range strings.Lines(porcelain) {
		if strings.HasPrefix(line, "!\t") {
			return true
		}
	}
	return false
}

func (r *Repo) runHook(stage string) {
	if r.hook != nil {
		r.hook(stage)
	}
}
