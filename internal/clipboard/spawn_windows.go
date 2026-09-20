package clipboard

import (
	"errors"
	"os"
	"os/exec"
)

var ErrOSC52ClearUnsupported = errors.New("osc52 clear is not supported on windows")

func attach(_ *exec.Cmd, job ClearJob) error {
	if job.Mode == ModeOSC52 {
		return ErrOSC52ClearUnsupported
	}
	return nil
}

func ChildTTY() *os.File {
	return nil
}
