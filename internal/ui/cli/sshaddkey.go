package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/spf13/cobra"
)

const sshAddKeyLong = `Save an existing private key in the vault.

A key already in ~/.ssh is pushed, also when no ssh config uses it or an
earlier push skipped it. A key outside ~/.ssh is copied to ~/.ssh/<name>
(the file name unless --name is set) and pushed. A .pub file next to the key
is added too. Files that are not private keys, links and files over 64 KiB
are refused. An existing file with other content is never overwritten.`

type jsonSSHAdded struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

type jsonSSHAddKey struct {
	Dir    string         `json:"dir"`
	Path   string         `json:"path"`
	Files  []jsonSSHAdded `json:"files"`
	Unused bool           `json:"unused"`
	Note   string         `json:"note,omitzero"`
}

func sshAddKeyCmd(env Env) *cobra.Command {
	var name string
	c := &cobra.Command{
		Use:   "add-key <file>",
		Short: "Save an existing private key in ~/.ssh and the vault",
		Long:  sshAddKeyLong,
		Args:  cobra.ExactArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			res, err := s.SSH().AddKey(args[0], name)
			if err != nil {
				return err
			}
			return env.printSSHAddKey(res)
		}),
	}
	c.Flags().StringVar(&name, "name", "", "file name in ~/.ssh for a key outside it, like work_key")
	return c
}

func (e Env) printSSHAddKey(res app.SSHAddKeyResult) error {
	if e.jsonMode() {
		return e.writeJSON(toJSONSSHAddKey(res))
	}
	same := true
	for _, f := range res.Files {
		same = same && f.Action == app.SSHNoChange
		if err := e.printSSHLine(f, sshActionColumn); err != nil {
			return err
		}
	}
	if res.PubNote != "" {
		e.note("%s", res.PubNote)
	}
	if same {
		e.note("%s is already in ~/.ssh and the vault", res.Rel)
	}
	if res.Unused {
		e.note("not used by ~/.ssh/config yet, add IdentityFile ~/.ssh/%s to a host or run creds ssh add-host <group/name> --host <host> --key %s", res.Rel, res.Rel)
	}
	return nil
}

func toJSONSSHAddKey(res app.SSHAddKeyResult) jsonSSHAddKey {
	out := jsonSSHAddKey{Dir: res.Dir, Path: res.Rel, Files: []jsonSSHAdded{}, Unused: res.Unused, Note: res.PubNote}
	for _, f := range res.Files {
		out.Files = append(out.Files, jsonSSHAdded{Path: f.Rel, Action: string(f.Action)})
	}
	return out
}
