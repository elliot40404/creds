package cli

import (
	"os"

	"github.com/elliot40404/creds/internal/detach"
)

const sshCommandEnv = "GIT_SSH_COMMAND"

func spawnSync() error {
	return detach.Self(quietEnv(os.Environ(), os.Getenv(sshCommandEnv)), "sync", "--quiet")
}

func quietEnv(environ []string, ssh string) []string {
	if ssh == "" {
		ssh = "ssh"
	}
	return append(environ,
		"GIT_TERMINAL_PROMPT=0",
		sshCommandEnv+"="+ssh+" -o BatchMode=yes",
		"SSH_ASKPASS_REQUIRE=never",
		"GCM_INTERACTIVE=never",
	)
}
