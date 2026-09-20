package detach

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

const detached = windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP

func attr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: detached | windows.CREATE_BREAKAWAY_FROM_JOB}
}

func start(cmd *exec.Cmd) (*os.Process, error) {
	err := cmd.Start()
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return cmd.Process, err
	}
	again := &exec.Cmd{
		Path:        cmd.Path,
		Args:        cmd.Args,
		Env:         cmd.Env,
		Dir:         cmd.Dir,
		Stdin:       cmd.Stdin,
		Stdout:      cmd.Stdout,
		Stderr:      cmd.Stderr,
		SysProcAttr: &syscall.SysProcAttr{CreationFlags: detached},
	}
	err = again.Start()
	return again.Process, err
}
