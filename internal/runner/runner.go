package runner

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
)

var ErrNoCommand = errors.New("no command given")

type Cmd struct {
	Args   []string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func Run(c Cmd) (int, error) {
	if len(c.Args) == 0 || c.Args[0] == "" {
		return 0, ErrNoCommand
	}
	path, err := exec.LookPath(c.Args[0])
	if err != nil {
		return 0, err
	}
	cmd := &exec.Cmd{
		Path:   path,
		Args:   c.Args,
		Env:    append(os.Environ(), c.Env...),
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	err = cmd.Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitCode(exit.ExitCode()), nil
	}
	return 0, err
}

func exitCode(code int) int {
	if code < 0 {
		return 1
	}
	return code
}

func IsNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}
