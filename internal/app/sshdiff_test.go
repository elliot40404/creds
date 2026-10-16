package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/textdiff"
)

func TestSSHDiffKeyHidesContent(t *testing.T) {
	t.Parallel()
	key := &sshdir.File{Rel: "id", Kind: sshdir.KindPrivateKey, Data: []byte("secret one")}
	other := &sshdir.File{Rel: "id", Kind: sshdir.KindPrivateKey, Data: []byte("secret two")}
	for _, f := range []SSHFileStatus{{Disk: key, Vault: other}, {Disk: key}, {Vault: other}} {
		if d := f.Diff(); !d.Key || len(d.Lines) != 0 {
			t.Fatalf("Diff() = %+v, want key only", d)
		}
	}
}

func TestSSHDiffLines(t *testing.T) {
	t.Parallel()
	f := SSHFileStatus{
		Disk:  &sshdir.File{Rel: "config", Data: []byte("Host b\n")},
		Vault: &sshdir.File{Rel: "config", Data: []byte("Host a\n")},
	}
	d := f.Diff()
	want := []textdiff.Line{{Op: textdiff.Del, Text: "Host b"}, {Op: textdiff.Add, Text: "Host a"}}
	if d.Key || d.More != 0 || fmt.Sprint(d.Lines) != fmt.Sprint(want) {
		t.Fatalf("Diff() = %+v, want %v", d, want)
	}
}

func TestSSHDiffCapsLines(t *testing.T) {
	t.Parallel()
	f := SSHFileStatus{Vault: &sshdir.File{Rel: "config", Data: []byte(strings.Repeat("x\n", sshDiffMax+5))}}
	if d := f.Diff(); len(d.Lines) != sshDiffMax || d.More != 5 {
		t.Fatalf("Diff() lines %d more %d, want %d and 5", len(d.Lines), d.More, sshDiffMax)
	}
}
