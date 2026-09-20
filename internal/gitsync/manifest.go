package gitsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"

	"github.com/elliot40404/creds/internal/anchor"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/manifest"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

var ErrUnverified = errors.New("gitsync: remote vault state is not authenticated")

func (r *Repo) treeManifest(ctx context.Context, t tree, id *crypto.Identity) (manifest.Manifest, error) {
	hashes, blobs, err := r.hashTree(ctx, t)
	if err != nil {
		return manifest.Manifest{}, err
	}
	m, err := manifest.Open(id, blobs[t[vaultfiles.ManifestFile]])
	if err != nil {
		return m, fmt.Errorf("%w: %w", ErrUnverified, err)
	}
	if err := manifest.Match(m, hashes); err != nil {
		return m, fmt.Errorf("%w: %w", ErrUnverified, err)
	}
	return m, nil
}

func (r *Repo) hashTree(ctx context.Context, t tree) (map[string]string, map[string][]byte, error) {
	want := vaultfiles.Required()
	shas := make([]string, 0, len(want))
	for _, p := range want {
		sha, ok := t[p]
		if !ok {
			return nil, nil, fmt.Errorf("%w: %s", ErrIncomplete, p)
		}
		shas = append(shas, sha)
	}
	blobs, err := r.readBlobs(ctx, shas)
	if err != nil {
		return nil, nil, err
	}
	hashes := map[string]string{}
	for _, p := range vaultfiles.Covered() {
		sum := sha256.Sum256(blobs[t[p]])
		hashes[p] = hex.EncodeToString(sum[:])
	}
	return hashes, blobs, nil
}

func (r *Repo) verifyTree(ctx context.Context, opts SyncOptions, rev string) (manifest.Manifest, error) {
	if opts.Identity == nil || rev == "" {
		return manifest.Manifest{}, nil
	}
	t, err := r.readTree(ctx, rev)
	if err != nil {
		return manifest.Manifest{}, err
	}
	return r.treeManifest(ctx, t, opts.Identity)
}

func (r *Repo) HeadManifest(ctx context.Context, id *crypto.Identity) (manifest.Manifest, bool, error) {
	head, err := r.Head(ctx)
	if err != nil || head == "" {
		return manifest.Manifest{}, false, err
	}
	t, err := r.readTree(ctx, head)
	if err != nil {
		return manifest.Manifest{}, false, err
	}
	if _, ok := t[vaultfiles.ManifestFile]; !ok {
		return manifest.Manifest{}, false, nil
	}
	m, err := r.treeManifest(ctx, t, id)
	return m, err == nil, err
}

func (r *Repo) verifyAdopt(ctx context.Context, opts SyncOptions, rev string) error {
	m, err := r.verifyTree(ctx, opts, rev)
	if err != nil || opts.Identity == nil || rev == "" {
		return err
	}
	return checkAnchor(opts, m)
}

func checkAnchor(opts SyncOptions, m manifest.Manifest) error {
	if opts.AnchorPath == "" {
		return nil
	}
	a, err := anchor.Load(opts.AnchorPath)
	if errors.Is(err, anchor.ErrNoAnchor) {
		return nil
	}
	if err != nil {
		return err
	}
	return anchor.Check(a, m.Generation)
}

func (r *Repo) verifyMerge(ctx context.Context, opts SyncOptions, trees [3]tree) error {
	theirs, err := r.treeManifest(ctx, trees[2], opts.Identity)
	if err != nil {
		return err
	}
	if len(trees[0]) == 0 {
		return checkAnchor(opts, theirs)
	}
	sha, ok := trees[0][vaultfiles.ManifestFile]
	if !ok {
		return nil
	}
	blobs, err := r.readBlobs(ctx, []string{sha})
	if err != nil {
		return err
	}
	base, err := manifest.Open(opts.Identity, blobs[sha])
	if err != nil {
		return err
	}
	return checkBase(base, theirs)
}

func checkBase(base, theirs manifest.Manifest) error {
	if theirs.Generation > base.Generation {
		return nil
	}
	b, err := base.ID()
	if err != nil {
		return err
	}
	t, err := theirs.ID()
	if err != nil || t == b {
		return err
	}
	return fmt.Errorf("%w: generation %d, merge base %d", anchor.ErrRollback, theirs.Generation, base.Generation)
}

func (r *Repo) advanceAnchor(ctx context.Context, opts SyncOptions, res SyncResult) error {
	if opts.Identity == nil || opts.AnchorPath == "" {
		return nil
	}
	if res != FastForwarded && res != Merged && res != Pushed {
		return nil
	}
	head, err := r.Head(ctx)
	if err != nil || head == "" {
		return err
	}
	m, err := r.verifyTree(ctx, opts, head)
	if err != nil {
		return err
	}
	return anchor.Record(opts.AnchorPath, m)
}

func (r *Repo) writeMergeManifest(ctx context.Context, opts SyncOptions, trees [3]tree, files tree) error {
	ours, err := r.treeManifest(ctx, trees[1], opts.Identity)
	if err != nil {
		return err
	}
	theirs, err := r.treeManifest(ctx, trees[2], opts.Identity)
	if err != nil {
		return err
	}
	hashes, _, err := r.hashTree(ctx, withManifest(files, trees[1]))
	if err != nil {
		return err
	}
	m, err := manifest.Next([]manifest.Manifest{ours, theirs}, hashes)
	if err != nil {
		return err
	}
	data, err := manifest.Seal(opts.Identity, m)
	if err != nil {
		return err
	}
	sha, err := r.writeBlob(ctx, data)
	if err != nil {
		return err
	}
	files[vaultfiles.ManifestFile] = sha
	return nil
}

func withManifest(files, fallback tree) tree {
	if _, ok := files[vaultfiles.ManifestFile]; ok {
		return files
	}
	out := tree{}
	maps.Copy(out, files)
	out[vaultfiles.ManifestFile] = fallback[vaultfiles.ManifestFile]
	return out
}
