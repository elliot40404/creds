package gitsync

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

const mergeMessage = "Merge remote changes"

var ErrConflict = errors.New("gitsync: merge conflict")

type SyncOptions struct {
	Identity    *crypto.Identity
	PreferFile  map[string]vault.Side
	PreferEntry map[uuid.UUID]vault.Side
	AnchorPath  string
}

type Conflict struct {
	File  string
	Entry *vault.Conflict
}

type ConflictError struct {
	Conflicts []Conflict
	Live      []Conflict
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("gitsync: %d merge conflicts", len(e.Conflicts))
}

func (e *ConflictError) Unwrap() error { return ErrConflict }

func (r *Repo) merge(ctx context.Context, opts SyncOptions, local, remote string) (SyncResult, error) {
	trees, err := r.mergeTrees(ctx, local, remote)
	if err != nil {
		return UpToDate, err
	}
	if err := r.verifyMerge(ctx, opts, trees); err != nil {
		return UpToDate, err
	}
	sets, err := r.bucketSets(ctx, trees)
	if err != nil {
		return UpToDate, err
	}
	files, conflicts := mergeFiles(trees, opts.PreferFile)
	merged, entries, err := vault.MergeVault(opts.Identity, sets[0], sets[1], sets[2], opts.PreferEntry)
	if err != nil {
		return UpToDate, err
	}
	if conflicts = append(conflicts, entryConflicts(entries)...); len(conflicts) > 0 {
		return UpToDate, conflictError(opts, trees, sets, conflicts)
	}
	if err := r.storeBuckets(ctx, trees[1], files, merged); err != nil {
		return UpToDate, err
	}
	if err := r.writeMergeManifest(ctx, opts, trees, files); err != nil {
		return UpToDate, err
	}
	if err := r.checkIncoming(ctx, trees[1], files); err != nil {
		return UpToDate, err
	}
	if err := r.commitMerge(ctx, files, local, remote); err != nil {
		return UpToDate, err
	}
	if _, err := r.push(ctx); err != nil {
		return UpToDate, err
	}
	return Merged, nil
}

func (r *Repo) mergeTrees(ctx context.Context, local, remote string) ([3]tree, error) {
	var trees [3]tree
	base, err := r.mergeBase(ctx, local, remote)
	if err != nil {
		return trees, err
	}
	for i, rev := range []string{base, local, remote} {
		if trees[i], err = r.readTree(ctx, rev); err != nil {
			return trees, err
		}
	}
	return trees, r.checkIncoming(ctx, trees[1], trees[2])
}

func (r *Repo) mergeBase(ctx context.Context, a, b string) (string, error) {
	out, err := r.git.Run(ctx, "merge-base", a, b)
	if exitCode(err) == 1 {
		return "", nil
	}
	return strings.TrimSpace(string(out)), err
}

func mergeFiles(trees [3]tree, prefer map[string]vault.Side) (tree, []Conflict) {
	base, ours, theirs := trees[0], trees[1], trees[2]
	out := tree{}
	var conflicts []Conflict
	for _, p := range slices.Sorted(maps.Keys(union(ours, theirs))) {
		if strings.HasPrefix(p, entriesPath) || p == vaultfiles.ManifestFile {
			continue
		}
		sha, ok := pick(base[p], ours[p], theirs[p], prefer[p])
		if !ok {
			conflicts = append(conflicts, Conflict{File: p})
			continue
		}
		if sha != "" {
			out[p] = sha
		}
	}
	return out, conflicts
}

func pick(base, ours, theirs string, prefer vault.Side) (string, bool) {
	switch {
	case prefer == vault.Mine, prefer != vault.Theirs && (ours == theirs || base == theirs):
		return ours, true
	case prefer == vault.Theirs, base == ours:
		return theirs, true
	}
	return "", false
}

func union(trees ...tree) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range trees {
		for p := range t {
			out[p] = struct{}{}
		}
	}
	return out
}

func entryConflicts(conflicts []vault.Conflict) []Conflict {
	out := make([]Conflict, 0, len(conflicts))
	for _, c := range conflicts {
		out = append(out, Conflict{Entry: &c})
	}
	return out
}

func conflictError(opts SyncOptions, trees [3]tree, sets [3]vault.BucketSet, remaining []Conflict) error {
	live := remaining
	if len(opts.PreferFile)+len(opts.PreferEntry) > 0 {
		_, files := mergeFiles(trees, nil)
		_, entries, err := vault.MergeVault(opts.Identity, sets[0], sets[1], sets[2], nil)
		if err != nil {
			return err
		}
		live = append(files, entryConflicts(entries)...)
	}
	return &ConflictError{Conflicts: remaining, Live: live}
}

func (r *Repo) storeBuckets(ctx context.Context, ours, files tree, merged vault.BucketSet) error {
	for p, sha := range ours {
		if strings.HasPrefix(p, entriesPath) {
			files[p] = sha
		}
	}
	for name, data := range merged {
		sha, err := r.writeBlob(ctx, data)
		if err != nil {
			return err
		}
		files[entriesPath+name] = sha
	}
	return nil
}

func (r *Repo) bucketSets(ctx context.Context, trees [3]tree) ([3]vault.BucketSet, error) {
	var sets [3]vault.BucketSet
	var shas []string
	for _, t := range trees {
		for p, sha := range t {
			if strings.HasPrefix(p, entriesPath) && !slices.Contains(shas, sha) {
				shas = append(shas, sha)
			}
		}
	}
	blobs, err := r.readBlobs(ctx, shas)
	if err != nil {
		return sets, err
	}
	for i, t := range trees {
		sets[i] = vault.BucketSet{}
		for p, sha := range t {
			if name, ok := strings.CutPrefix(p, entriesPath); ok {
				sets[i][name] = blobs[sha]
			}
		}
	}
	return sets, nil
}

func (r *Repo) commitMerge(ctx context.Context, files tree, local, remote string) error {
	treeSHA, err := r.writeTree(ctx, files)
	if err != nil {
		return err
	}
	args := []string{"commit-tree", treeSHA, "-p", local, "-p", remote, "-m", mergeMessage}
	if r.git.signsCommits() {
		args = append(args, "-S")
	}
	commit, err := r.pipe(ctx, nil, args...)
	if err != nil {
		return err
	}
	_, err = r.git.Run(ctx, "merge", "-q", "--ff-only", commit)
	return err
}
