package cli

import (
	"github.com/elliot40404/creds/internal/app"
	"github.com/spf13/cobra"
)

const sshAddHostLong = `Add a host to ~/.ssh/config.d/<group/name> and save it in the vault.

The file holds one Host block named after the last part of the name, so
beam/data_new adds Host data_new. --key names a private key file in ~/.ssh,
it adds IdentityFile and IdentitiesOnly yes. An existing file is never
overwritten: edit it in ~/.ssh and run creds ssh push. ~/.ssh/config is
never changed, when it does not include the new file the Include line to add
is printed.`

type jsonSSHAddHost struct {
	Dir     string `json:"dir"`
	Path    string `json:"path"`
	Alias   string `json:"alias"`
	Include string `json:"include,omitzero"`
}

func sshAddHostCmd(env Env) *cobra.Command {
	var h app.SSHHost
	c := &cobra.Command{
		Use:   "add-host <group/name>",
		Short: "Add a host file to ~/.ssh/config.d and the vault",
		Long:  sshAddHostLong,
		Args:  cobra.ExactArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			h.Name = args[0]
			res, err := s.SSH().AddHost(h)
			if err != nil {
				return err
			}
			return env.printSSHAddHost(res)
		}),
	}
	c.Flags().StringVar(&h.Host, "host", "", "host name or IP to connect to")
	c.Flags().StringVar(&h.User, "user", "", "user to log in as")
	c.Flags().IntVar(&h.Port, "port", 0, "port, 22 when not set")
	c.Flags().StringVar(&h.Key, "key", "", "private key file in ~/.ssh, like id_ed25519")
	return c
}

func (e Env) printSSHAddHost(res app.SSHAddHostResult) error {
	if e.jsonMode() {
		return e.writeJSON(jsonSSHAddHost{Dir: res.Dir, Path: res.Rel, Alias: res.Alias, Include: res.Include})
	}
	if err := e.printSSHLine(app.SSHFileResult{Rel: res.Rel, Action: app.SSHCreated}, sshActionColumn); err != nil {
		return err
	}
	if res.Include != "" {
		e.note("~/.ssh/config does not include it, add this line at the top of it: %s", res.Include)
		return nil
	}
	e.note("added, run ssh %s to connect", res.Alias)
	return nil
}
