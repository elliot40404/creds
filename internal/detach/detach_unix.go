//go:build !windows

package detach

import (
	"os"
	"os/exec"
	"syscall"
)

func attr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func start(cmd *exec.Cmd) (*os.Process, error) {
	err := cmd.Start()
	return cmd.Process, err
}
