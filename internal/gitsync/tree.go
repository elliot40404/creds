package gitsync

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

const (
	blobMode    = "100644"
	treeMode    = "040000"
	entriesPath = vaultfiles.EntriesDir + "/"
)

type tree map[string]string

func (r *Repo) readTree(ctx context.Context, rev string) (tree, error) {
	t := tree{}
	if rev == "" {
		return t, nil
	}
	out, err := r.git.RunLimit(ctx, maxTreeOutput, "ls-tree output", "ls-tree", "-r", "-z", "-l", "--full-tree", rev)
	if err != nil {
		return nil, err
	}
	for rec := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if rec == "" {
			continue
		}
		path, sha, err := parseTreeRecord(rec)
		if err != nil {
			return nil, err
		}
		t[path] = sha
	}
	return t, nil
}

func parseTreeRecord(rec string) (string, string, error) {
	meta, path, ok := strings.Cut(rec, "\t")
	fields := strings.Fields(meta)
	if !ok || len(fields) != 4 {
		return "", "", fmt.Errorf("gitsync: bad tree record %q", rec)
	}
	if !vaultfiles.Allowed(path) || fields[0] != blobMode || fields[1] != "blob" {
		return "", "", fmt.Errorf("%w: %s %s", ErrNotAllowed, fields[0], path)
	}
	size, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("gitsync: bad blob size %q", fields[3])
	}
	if size > vaultfiles.MaxFileSize {
		return "", "", tooLarge(path)
	}
	return path, fields[2], nil
}

func (r *Repo) readBlobs(ctx context.Context, shas []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	if len(shas) == 0 {
		return out, nil
	}
	var in, buf bytes.Buffer
	for _, s := range shas {
		in.WriteString(s + "\n")
	}
	w := &limitWriter{w: &buf, left: maxBlobTotal, what: "remote entry data"}
	if err := r.git.run(ctx, &in, w, []string{"cat-file", "--batch"}); err != nil {
		return nil, err
	}
	br := bufio.NewReader(&buf)
	for _, s := range shas {
		data, err := readBlob(br)
		if err != nil {
			return nil, fmt.Errorf("gitsync: read blob %s: %w", s, err)
		}
		out[s] = data
	}
	return out, nil
}

func readBlob(br *bufio.Reader) ([]byte, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(line)
	if len(fields) != 3 || fields[1] != "blob" {
		return nil, fmt.Errorf("unexpected object %q", strings.TrimSpace(line))
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 {
		return nil, fmt.Errorf("bad object size %q", fields[2])
	}
	if size > vaultfiles.MaxFileSize {
		return nil, tooLarge("remote entry data")
	}
	data := make([]byte, size+1)
	if _, err := io.ReadFull(br, data); err != nil {
		return nil, err
	}
	return data[:size], nil
}

func (r *Repo) writeBlob(ctx context.Context, data []byte) (string, error) {
	return r.pipe(ctx, data, "hash-object", "-w", "--stdin")
}

func (r *Repo) writeTree(ctx context.Context, files tree) (string, error) {
	var root, entries bytes.Buffer
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if name, ok := strings.CutPrefix(p, entriesPath); ok {
			writeTreeRecord(&entries, blobMode, "blob", files[p], name)
		} else {
			writeTreeRecord(&root, blobMode, "blob", files[p], p)
		}
	}
	if entries.Len() > 0 {
		sha, err := r.pipe(ctx, entries.Bytes(), "mktree", "-z")
		if err != nil {
			return "", err
		}
		writeTreeRecord(&root, treeMode, "tree", sha, vaultfiles.EntriesDir)
	}
	return r.pipe(ctx, root.Bytes(), "mktree", "-z")
}

func writeTreeRecord(buf *bytes.Buffer, mode, kind, sha, name string) {
	buf.WriteString(mode + " " + kind + " " + sha + "\t" + name + "\x00")
}

func (r *Repo) pipe(ctx context.Context, stdin []byte, args ...string) (string, error) {
	var out bytes.Buffer
	err := r.git.run(ctx, bytes.NewReader(stdin), &out, args)
	return strings.TrimSpace(out.String()), err
}
