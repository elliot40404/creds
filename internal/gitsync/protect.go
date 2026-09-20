package gitsync

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

var (
	ErrIncomplete = errors.New("gitsync: remote vault is missing files")
	ErrProtected  = errors.New("gitsync: remote changed the recovery file or vault recipient")
	ErrBadVersion = errors.New("gitsync: remote vault format version is not supported")
	ErrBadMeta    = errors.New("gitsync: remote vault meta is not valid")
)

type metaProbe struct {
	Recipient string `json:"recipient"`
}

func requireComplete(t tree) error {
	for _, p := range vaultfiles.Required() {
		if _, ok := t[p]; !ok {
			return fmt.Errorf("%w: %s", ErrIncomplete, p)
		}
	}
	return nil
}

func (r *Repo) revTrees(ctx context.Context, local, incoming string) (tree, tree, error) {
	lt, err := r.readTree(ctx, local)
	if err != nil {
		return nil, nil, err
	}
	it, err := r.readTree(ctx, incoming)
	if err != nil {
		return nil, nil, err
	}
	return lt, it, r.checkIncoming(ctx, lt, it)
}

func (r *Repo) checkIncoming(ctx context.Context, local, incoming tree) error {
	if err := requireComplete(incoming); err != nil {
		return err
	}
	if len(local) > 0 && local[vaultfiles.RecoveryFile] != incoming[vaultfiles.RecoveryFile] {
		return fmt.Errorf("%w: %s", ErrProtected, vaultfiles.RecoveryFile)
	}
	return r.checkMeta(ctx, local[vaultfiles.MetaFile], incoming[vaultfiles.MetaFile])
}

func (r *Repo) checkMeta(ctx context.Context, localSHA, incomingSHA string) error {
	if localSHA == incomingSHA {
		return nil
	}
	shas := []string{incomingSHA}
	if localSHA != "" {
		shas = append(shas, localSHA)
	}
	blobs, err := r.readBlobs(ctx, shas)
	if err != nil {
		return err
	}
	in, err := parseIncomingMeta(blobs[incomingSHA], localSHA == "")
	if err != nil || localSHA == "" {
		return err
	}
	var out metaProbe
	if json.Unmarshal(blobs[localSHA], &out) != nil || out.Recipient != in.Recipient {
		return fmt.Errorf("%w: %s", ErrProtected, vaultfiles.MetaFile)
	}
	return nil
}

func parseIncomingMeta(data []byte, fresh bool) (format.VaultMeta, error) {
	m, err := format.ParseMeta(data)
	switch {
	case err == nil:
		return m, nil
	case fresh && errors.Is(err, format.ErrOldVersion):
		return m, nil
	case errors.Is(err, format.ErrOldVersion), errors.Is(err, format.ErrUnknownVersion):
		return m, fmt.Errorf("%w: %w", ErrBadVersion, err)
	}
	return m, fmt.Errorf("%w: %w", ErrBadMeta, err)
}

func changedBuckets(local, incoming tree) map[string]string {
	out := map[string]string{}
	for p, sha := range incoming {
		if name, ok := strings.CutPrefix(p, entriesPath); ok && local[p] != sha {
			out[name] = sha
		}
	}
	return out
}

func (r *Repo) openBuckets(ctx context.Context, id *crypto.Identity, buckets map[string]string) error {
	shas := make([]string, 0, len(buckets))
	for _, sha := range buckets {
		shas = append(shas, sha)
	}
	blobs, err := r.readBlobs(ctx, shas)
	if err != nil {
		return err
	}
	set := vault.BucketSet{}
	for name, sha := range buckets {
		set[name] = blobs[sha]
	}
	_, conflicts, err := vault.MergeVault(id, nil, set, set, nil)
	if err == nil && len(conflicts) > 0 {
		err = fmt.Errorf("%w: duplicate entry path %s", vault.ErrBadBucket, conflicts[0].Paths[0])
	}
	return err
}
