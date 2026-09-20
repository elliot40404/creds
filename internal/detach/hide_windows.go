package detach

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}
