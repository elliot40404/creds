package clipboard

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fakeCall struct {
	path  string
	args  []string
	stdin string
}

type fakeTools struct {
	have  map[string]bool
	out   map[string]string
	fail  map[string]error
	calls []fakeCall
}

func (f *fakeTools) set(names ...string) toolSet {
	f.have = map[string]bool{}
	for _, n := range names {
		f.have[n] = true
	}
	return toolSet{
		tools:    platformTools("linux"),
		lookPath: f.lookPath,
		run:      f.run,
	}
}

func (f *fakeTools) lookPath(name string) (string, error) {
	if !f.have[name] {
		return "", exec.ErrNotFound
	}
	return "/fake/" + name, nil
}

func (f *fakeTools) run(path string, args []string, in io.Reader, out io.Writer) error {
	call := fakeCall{path: path, args: args}
	if in != nil {
		b, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		call.stdin = string(b)
	}
	f.calls = append(f.calls, call)
	name := filepath.Base(path)
	if out != nil {
		if _, err := io.WriteString(out, f.out[name]); err != nil {
			return err
		}
	}
	return f.fail[name]
}

type fakeExit int

func (e fakeExit) ExitCode() int { return int(e) }
func (e fakeExit) Error() string { return "exit" }

func TestFindPrefersWayland(t *testing.T) {
	var f fakeTools
	set := f.set("wl-copy", "wl-paste", "xclip", "xsel")
	found, err := set.find()
	if err != nil {
		t.Fatal(err)
	}
	if found.tool.copyCmd[0] != "wl-copy" {
		t.Fatalf("picked %q", found.tool.copyCmd[0])
	}
}

func TestFindSkipsHalfInstalled(t *testing.T) {
	var f fakeTools
	set := f.set("wl-copy", "xclip")
	found, err := set.find()
	if err != nil {
		t.Fatal(err)
	}
	if found.tool.copyCmd[0] != "xclip" {
		t.Fatalf("picked %q", found.tool.copyCmd[0])
	}
}

func TestFindNoToolNamesTools(t *testing.T) {
	var f fakeTools
	_, err := f.set().find()
	if !errors.Is(err, ErrNoTool) {
		t.Fatalf("err %v", err)
	}
	for _, name := range []string{"wl-copy", "xclip", "xsel"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("%q missing from %q", name, err)
		}
	}
}

func TestWriteSendsValueOnStdin(t *testing.T) {
	var f fakeTools
	set := f.set("xclip")
	const value = "s3cret ünï"
	if err := set.write(value); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("calls %d", len(f.calls))
	}
	call := f.calls[0]
	if call.stdin != value {
		t.Fatalf("stdin %q", call.stdin)
	}
	for _, arg := range call.args {
		if strings.Contains(arg, "s3cret") {
			t.Fatalf("value in args %v", call.args)
		}
	}
}

func TestReadReturnsOutput(t *testing.T) {
	f := fakeTools{out: map[string]string{"xsel": "hello"}}
	set := f.set("xsel")
	got, err := set.read()
	if err != nil || got != "hello" {
		t.Fatalf("read %q, %v", got, err)
	}
}

func TestReadEmptyClipboardExitOne(t *testing.T) {
	f := fakeTools{fail: map[string]error{"wl-paste": fakeExit(1)}}
	set := f.set("wl-copy", "wl-paste")
	got, err := set.read()
	if err != nil || got != "" {
		t.Fatalf("read %q, %v", got, err)
	}
}

func TestReadRealFailure(t *testing.T) {
	f := fakeTools{fail: map[string]error{"wl-paste": fakeExit(2)}}
	set := f.set("wl-copy", "wl-paste")
	if _, err := set.read(); err == nil {
		t.Fatal("want error")
	}
}

func TestClearUsesClearFlag(t *testing.T) {
	var f fakeTools
	set := f.set("wl-copy", "wl-paste")
	if err := set.clear(); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.calls[0].args[0] != "--clear" {
		t.Fatalf("calls %+v", f.calls)
	}
}

func TestClearWritesEmpty(t *testing.T) {
	var f fakeTools
	set := f.set("xclip")
	if err := set.clear(); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.calls[0].stdin != "" {
		t.Fatalf("calls %+v", f.calls)
	}
}

func TestPlatformToolsDarwin(t *testing.T) {
	tools := platformTools("darwin")
	if len(tools) != 1 || tools[0].copyCmd[0] != "pbcopy" || tools[0].pasteCmd[0] != "pbpaste" {
		t.Fatalf("tools %+v", tools)
	}
}

func TestRunToolRealBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a posix shell")
	}
	path, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat not found")
	}
	var out strings.Builder
	if err := runTool(path, nil, strings.NewReader("piped"), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "piped" {
		t.Fatalf("out %q", out.String())
	}
}

func TestSystemToolSetUsesPath(t *testing.T) {
	set := systemToolSet()
	if len(set.tools) == 0 || set.lookPath == nil || set.run == nil {
		t.Fatal("incomplete tool set")
	}
	if _, err := set.lookPath(string(os.PathSeparator) + "definitely-not-a-clipboard-tool"); err == nil {
		t.Fatal("want lookup error")
	}
}

func TestWriteFallsBackWhenWaylandFails(t *testing.T) {
	f := fakeTools{fail: map[string]error{"wl-copy": errors.New("no wayland server")}}
	set := f.set("wl-copy", "wl-paste", "xclip")
	if err := set.write("v"); err != nil {
		t.Fatal(err)
	}
	last := f.calls[len(f.calls)-1]
	if filepath.Base(last.path) != "xclip" || last.stdin != "v" {
		t.Fatalf("calls %+v", f.calls)
	}
}

func TestDisplayPicksTheToolOrder(t *testing.T) {
	var f fakeTools
	set := f.set("wl-copy", "wl-paste", "xclip")
	set.getenv = func(k string) string {
		if k == "DISPLAY" {
			return ":0"
		}
		return ""
	}
	found, err := set.find()
	if err != nil || found.tool.copyCmd[0] != "xclip" {
		t.Fatalf("picked %+v %v", found, err)
	}
	set.getenv = func(k string) string {
		if k == "WAYLAND_DISPLAY" || k == "DISPLAY" {
			return "x"
		}
		return ""
	}
	if found, err := set.find(); err != nil || found.tool.copyCmd[0] != "wl-copy" {
		t.Fatalf("picked %+v %v", found, err)
	}
}
