//go:build !windows

package detach

import "os/exec"

func Hide(*exec.Cmd) {}
