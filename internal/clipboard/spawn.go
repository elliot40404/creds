package clipboard

import (
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/elliot40404/creds/internal/detach"
)

var ErrNoTTY = errors.New("osc52 clear needs a terminal")

type ClearJob struct {
	After time.Duration
	Mode  Mode
	Hash  string
	TTY   *os.File
}

func Spawn(job ClearJob) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd, in, err := clearCmd(exe, job)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	return detach.Start(cmd)
}

func clearCmd(exe string, job ClearJob) (*exec.Cmd, *os.File, error) {
	if job.Mode != ModeNative && job.Mode != ModeOSC52 {
		return nil, nil, ErrBadMode
	}
	if job.Mode == ModeOSC52 && job.TTY == nil {
		return nil, nil, ErrNoTTY
	}
	cmd := detach.Command(exe, ClearCommand, "--after", job.After.String(), "--mode", job.Mode.String())
	if err := attach(cmd, job); err != nil {
		return nil, nil, err
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	_, err = w.WriteString(job.Hash + "\n")
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = r.Close()
		return nil, nil, err
	}
	cmd.Stdin = r
	return cmd, r, nil
}
