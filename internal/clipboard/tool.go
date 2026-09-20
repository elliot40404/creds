package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var ErrNoTool = errors.New("no clipboard tool found")

type clipTool struct {
	copyCmd   []string
	pasteCmd  []string
	clearCmd  []string
	emptyExit bool
}

var waylandTool = clipTool{
	copyCmd:   []string{"wl-copy"},
	pasteCmd:  []string{"wl-paste", "--no-newline"},
	clearCmd:  []string{"--clear"},
	emptyExit: true,
}

var x11Tools = []clipTool{
	{
		copyCmd:   []string{"xclip", "-selection", "clipboard"},
		pasteCmd:  []string{"xclip", "-selection", "clipboard", "-o"},
		emptyExit: true,
	},
	{
		copyCmd:   []string{"xsel", "--clipboard", "--input"},
		pasteCmd:  []string{"xsel", "--clipboard", "--output"},
		emptyExit: true,
	},
}

var darwinTool = clipTool{
	copyCmd:  []string{"pbcopy"},
	pasteCmd: []string{"pbpaste"},
}

func platformTools(goos string) []clipTool {
	if goos == "darwin" {
		return []clipTool{darwinTool}
	}
	return append([]clipTool{waylandTool}, x11Tools...)
}

type toolSet struct {
	tools    []clipTool
	lookPath func(string) (string, error)
	run      func(path string, args []string, in io.Reader, out io.Writer) error
	getenv   func(string) string
}

func systemToolSet() toolSet {
	return toolSet{
		tools:    platformTools(runtime.GOOS),
		lookPath: exec.LookPath,
		run:      runTool,
		getenv:   os.Getenv,
	}
}

func (t toolSet) ordered() []clipTool {
	if t.getenv == nil || t.getenv("WAYLAND_DISPLAY") != "" || t.getenv("DISPLAY") == "" {
		return t.tools
	}
	var x11, rest []clipTool
	for _, tool := range t.tools {
		if tool.copyCmd[0] == waylandTool.copyCmd[0] {
			rest = append(rest, tool)
		} else {
			x11 = append(x11, tool)
		}
	}
	return append(x11, rest...)
}

type foundTool struct {
	tool      clipTool
	copyPath  string
	pastePath string
}

func (t toolSet) find() (foundTool, error) {
	found := t.available()
	if len(found) == 0 {
		return foundTool{}, fmt.Errorf("%w: install %s", ErrNoTool, t.names())
	}
	return found[0], nil
}

func (t toolSet) available() []foundTool {
	var out []foundTool
	for _, tool := range t.ordered() {
		copyPath, err := t.lookPath(tool.copyCmd[0])
		if err != nil {
			continue
		}
		pastePath, err := t.lookPath(tool.pasteCmd[0])
		if err != nil {
			continue
		}
		out = append(out, foundTool{tool: tool, copyPath: copyPath, pastePath: pastePath})
	}
	return out
}

func (t toolSet) each(fn func(foundTool) error) error {
	found := t.available()
	if len(found) == 0 {
		return fmt.Errorf("%w: install %s", ErrNoTool, t.names())
	}
	var errs []error
	for _, f := range found {
		err := fn(f)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (t toolSet) names() string {
	names := make([]string, 0, len(t.tools))
	for _, tool := range t.tools {
		names = append(names, tool.copyCmd[0]+"/"+tool.pasteCmd[0])
	}
	return strings.Join(names, ", ")
}

func (t toolSet) read() (string, error) {
	var value string
	err := t.each(func(found foundTool) error {
		var out bytes.Buffer
		err := t.run(found.pastePath, found.tool.pasteCmd[1:], nil, &out)
		if err != nil && !emptyRead(found.tool, out.Len(), err) {
			return fmt.Errorf("%s: %w", found.tool.pasteCmd[0], err)
		}
		value = out.String()
		return nil
	})
	return value, err
}

func emptyRead(tool clipTool, n int, err error) bool {
	return tool.emptyExit && n == 0 && exitCode(err) == 1
}

func (t toolSet) write(value string) error {
	return t.each(func(found foundTool) error {
		return t.copyWith(found, found.tool.copyCmd[1:], strings.NewReader(value))
	})
}

func (t toolSet) clear() error {
	return t.each(func(found foundTool) error {
		if found.tool.clearCmd == nil {
			return t.copyWith(found, found.tool.copyCmd[1:], strings.NewReader(""))
		}
		return t.copyWith(found, found.tool.clearCmd, nil)
	})
}

func (t toolSet) copyWith(found foundTool, args []string, in io.Reader) error {
	if err := t.run(found.copyPath, args, in, nil); err != nil {
		return fmt.Errorf("%s: %w", found.tool.copyCmd[0], err)
	}
	return nil
}

func runTool(path string, args []string, in io.Reader, out io.Writer) error {
	cmd := exec.Command(path, args...)
	cmd.Stdin = in
	cmd.Stdout = out
	return cmd.Run()
}

type exitCoder interface {
	error
	ExitCode() int
}

func exitCode(err error) int {
	if coder, ok := errors.AsType[exitCoder](err); ok {
		return coder.ExitCode()
	}
	return -1
}
