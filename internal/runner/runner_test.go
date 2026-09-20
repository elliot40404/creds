package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

const helperEnv = "CREDS_RUNNER_HELPER"

func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		t.Skip("helper only")
	}
	var in bytes.Buffer
	_, _ = in.ReadFrom(os.Stdin)
	fmt.Printf("%s|%s|%s", os.Getenv("SECRET_VAR"), os.Getenv("SHADOW"), in.String())
	fmt.Fprint(os.Stderr, "to stderr")
	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
	os.Exit(code)
}

func helper(t *testing.T, exit string, env ...string) (int, string, string, error) {
	t.Helper()
	t.Setenv(helperEnv, "1")
	t.Setenv("HELPER_EXIT", exit)
	t.Setenv("SHADOW", "parent")
	var out, errb bytes.Buffer
	code, err := Run(Cmd{
		Args:   []string{os.Args[0], "-test.run=^TestHelperProcess$"},
		Env:    env,
		Stdin:  strings.NewReader("from stdin"),
		Stdout: &out,
		Stderr: &errb,
	})
	return code, out.String(), errb.String(), err
}

func TestRunPassesEnvAndStdio(t *testing.T) {
	code, out, errOut, err := helper(t, "0", "SECRET_VAR=s3cret", "SHADOW=child")
	if err != nil || code != 0 {
		t.Fatalf("code %d err %v", code, err)
	}
	if out != "s3cret|child|from stdin" || errOut != "to stderr" {
		t.Fatalf("out %q err %q", out, errOut)
	}
	if os.Getenv("SECRET_VAR") != "" {
		t.Fatal("parent env changed")
	}
}

func TestRunExitCode(t *testing.T) {
	code, _, _, err := helper(t, "7")
	if err != nil || code != 7 {
		t.Fatalf("code %d err %v", code, err)
	}
}

func TestRunErrors(t *testing.T) {
	if _, err := Run(Cmd{}); !errors.Is(err, ErrNoCommand) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := Run(Cmd{Args: []string{""}}); !errors.Is(err, ErrNoCommand) {
		t.Fatalf("blank: %v", err)
	}
	if _, err := Run(Cmd{Args: []string{"creds-no-such-binary-xyz"}}); err == nil {
		t.Fatal("missing binary ran")
	}
	if exitCode(-1) != 1 {
		t.Fatal("signal exit not mapped")
	}
}
