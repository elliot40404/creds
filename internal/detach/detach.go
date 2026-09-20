package detach

import (
	"os"
	"os/exec"
)

func Command(exe string, args ...string) *exec.Cmd {
	cmd := &exec.Cmd{Path: exe, Args: append([]string{exe}, args...)}
	cmd.SysProcAttr = attr()
	return cmd
}

func Self(env []string, args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := Command(exe, args...)
	cmd.Env = env
	return Start(cmd)
}

func Start(cmd *exec.Cmd) error {
	p, err := start(cmd)
	if err != nil {
		return err
	}
	return p.Release()
}
