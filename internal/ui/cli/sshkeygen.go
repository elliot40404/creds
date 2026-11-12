package cli

import (
	"errors"

	"github.com/elliot40404/creds/internal/app"
	"github.com/spf13/cobra"
)

const sshKeygenLong = `Make a new ed25519 key pair in ~/.ssh and save both files in the vault.

The private key goes to ~/.ssh/<name> and the public key to ~/.ssh/<name>.pub.
The comment at the end of the public key is the name unless --comment is set.
--passphrase asks for a passphrase twice at the terminal, it is never read
from a flag or the environment. An existing file is never overwritten.`

var errPassphraseTTY = errors.New("--passphrase needs an interactive terminal")

type jsonSSHKeygen struct {
	Dir       string         `json:"dir"`
	Path      string         `json:"path"`
	Files     []jsonSSHAdded `json:"files"`
	PublicKey string         `json:"public_key"`
}

func sshKeygenCmd(env Env) *cobra.Command {
	var k app.SSHKeygen
	run := env.run(func(s *app.Service, args []string) error {
		k.Name = args[0]
		res, err := s.SSH().Keygen(k)
		if err != nil {
			return err
		}
		return env.printSSHKeygen(res)
	})
	c := &cobra.Command{
		Use:   "keygen <name>",
		Short: "Make a new ed25519 key in ~/.ssh and the vault",
		Long:  sshKeygenLong,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if k.Passphrase && !isTTY(env.Prompter) {
				return errPassphraseTTY
			}
			return run(cmd, args)
		},
	}
	c.Flags().StringVar(&k.Comment, "comment", "", "comment at the end of the public key, like me@laptop")
	c.Flags().BoolVar(&k.Passphrase, "passphrase", false, "ask for a passphrase to protect the key")
	return c
}

func (e Env) printSSHKeygen(res app.SSHKeygenResult) error {
	if e.jsonMode() {
		return e.writeJSON(toJSONSSHKeygen(res))
	}
	for _, f := range res.Files {
		if err := e.printSSHLine(f, sshActionColumn); err != nil {
			return err
		}
	}
	e.note("public key: %s", res.PublicKey)
	e.note("add it to a host: creds ssh add-host <group/name> --host <host> --key %s", res.Rel)
	return nil
}

func toJSONSSHKeygen(res app.SSHKeygenResult) jsonSSHKeygen {
	out := jsonSSHKeygen{Dir: res.Dir, Path: res.Rel, Files: []jsonSSHAdded{}, PublicKey: res.PublicKey}
	for _, f := range res.Files {
		out.Files = append(out.Files, jsonSSHAdded{Path: f.Rel, Action: string(f.Action)})
	}
	return out
}
