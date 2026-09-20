package gitsync

import (
	"bufio"
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func TestParseTreeRecordRejectsOversizeBlob(t *testing.T) {
	t.Parallel()
	rec := blobMode + " blob " + strings.Repeat("a", 40) + " " + strconv.Itoa(vaultfiles.MaxFileSize+1) + "\tvault.json"
	if _, _, err := parseTreeRecord(rec); !errors.Is(err, fsutil.ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseTreeRecordRejectsBadSize(t *testing.T) {
	t.Parallel()
	rec := blobMode + " blob " + strings.Repeat("a", 40) + " nope\tvault.json"
	if _, _, err := parseTreeRecord(rec); err == nil || errors.Is(err, fsutil.ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
}

func batch(payload string) *bufio.Reader {
	sha := strings.Repeat("a", 40)
	return bufio.NewReader(strings.NewReader(sha + " blob " + strconv.Itoa(len(payload)) + "\n" + payload + "\n"))
}

func TestReadBlobReadsOneObject(t *testing.T) {
	t.Parallel()
	data, err := readBlob(batch("12345"))
	if err != nil || string(data) != "12345" {
		t.Fatalf("data %q err %v", data, err)
	}
}

func TestLimitWriterStopsAtTheCap(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := &limitWriter{w: &buf, left: 4, what: "test data"}
	if _, err := w.Write([]byte("abcd")); err != nil {
		t.Fatal(err)
	}
	_, err := w.Write([]byte("e"))
	if !errors.Is(err, fsutil.ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
	if buf.String() != "abcd" {
		t.Fatalf("buf = %q", buf.String())
	}
}
