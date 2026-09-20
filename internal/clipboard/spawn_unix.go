//go:build !windows

package clipboard

import (
	"os"
	"os/exec"
)

const childTTYFd = 3

func attach(cmd *exec.Cmd, job ClearJob) error {
	if job.Mode == ModeOSC52 {
		cmd.ExtraFiles = []*os.File{job.TTY}
	}
	return nil
}

func ChildTTY() *os.File {
	return os.NewFile(childTTYFd, "tty")
}
