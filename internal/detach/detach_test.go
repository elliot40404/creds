package detach

import (
	"slices"
	"testing"
)

func TestCommand(t *testing.T) {
	cmd := Command("/bin/creds", "sync", "--quiet")
	if cmd.Path != "/bin/creds" || !slices.Equal(cmd.Args, []string{"/bin/creds", "sync", "--quiet"}) {
		t.Fatalf("cmd = %v %v", cmd.Path, cmd.Args)
	}
	if cmd.SysProcAttr == nil || cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
		t.Fatal("not detached")
	}
}
