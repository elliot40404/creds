package editor

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
)

var ErrNoEditor = errors.New("no editor found, set EDITOR")

func Open(path string, in io.Reader, out io.Writer) error {
	name, args, err := command()
	if err != nil {
		return err
	}
	cmd := exec.Command(name, append(args, path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, out
	return cmd.Run()
}

func command() (string, []string, error) {
	for _, key := range []string{"VISUAL", "EDITOR"} {
		if parts := split(os.Getenv(key)); len(parts) > 0 {
			return parts[0], parts[1:], nil
		}
	}
	for _, name := range fallbacks() {
		if found, err := exec.LookPath(name); err == nil {
			return found, nil, nil
		}
	}
	return "", nil, ErrNoEditor
}

func fallbacks() []string {
	if runtime.GOOS == "windows" {
		return []string{"notepad.exe"}
	}
	return []string{"vi", "nano"}
}
