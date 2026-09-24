package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/device"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/spf13/cobra"
)

const maxKeyFile = 1 << 16

var errTrustSource = errors.New("pass exactly one of --touchid or --identity <file>")

type jsonDevice struct {
	Trusted   bool      `json:"trusted"`
	Plugin    string    `json:"plugin,omitzero"`
	Since     time.Time `json:"since,omitzero"`
	Confirmed time.Time `json:"confirmed,omitzero"`
	Expires   time.Time `json:"expires,omitzero"`
	Stale     bool      `json:"stale"`
}

func deviceCmd(env Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "device",
		Short: "Unlock this machine with Touch ID or another age plugin key",
		Long: "Trust this machine once with the master password, then unlock with a plugin key.\n" +
			"The key lives in device.json in the creds home, never in the vault or on the remote.",
		Args: cobra.NoArgs,
	}
	c.AddCommand(deviceTrustCmd(env), deviceUntrustCmd(env), deviceStatusCmd(env))
	return c
}

func deviceTrustCmd(env Env) *cobra.Command {
	var touchID bool
	var file string
	c := &cobra.Command{
		Use:   "trust",
		Short: "Trust this machine: unlock with Touch ID or an age plugin identity",
		Example: "  creds device trust --touchid\n" +
			"  creds device trust --identity ~/yubikey-identity.txt",
		Args: cobra.NoArgs,
	}
	c.RunE = env.run(func(s *app.Service, _ []string) error {
		key, err := trustKey(c.Context(), touchID, file)
		if err != nil {
			return err
		}
		if err := s.DeviceTrust(key); err != nil {
			return err
		}
		env.note("this device is trusted, the next unlock uses the plugin key")
		return nil
	})
	c.Flags().BoolVar(&touchID, "touchid", false, "make a Secure Enclave key with age-plugin-se (macOS)")
	c.Flags().StringVar(&file, "identity", "", "age plugin identity file, like one from age-plugin-yubikey")
	return c
}

func trustKey(ctx context.Context, touchID bool, file string) (string, error) {
	switch {
	case touchID == (file != ""):
		return "", errTrustSource
	case touchID:
		return device.TouchIDKey(ctx)
	}
	data, err := fsutil.ReadFileLimit(file, maxKeyFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func deviceUntrustCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "untrust",
		Short: "Stop unlocking this machine with the plugin key",
		Args:  cobra.NoArgs,
		RunE: env.withService(noPrompt{}, func(s *app.Service, _ []string) error {
			if err := s.DeviceUntrust(); err != nil {
				return err
			}
			env.note("this device is no longer trusted")
			return nil
		}),
	}
}

func deviceStatusCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether this machine unlocks with a plugin key",
		Args:  cobra.NoArgs,
		RunE: env.withService(noPrompt{}, func(s *app.Service, _ []string) error {
			st, err := s.DeviceStatus()
			if err != nil {
				return err
			}
			if env.jsonMode() {
				return env.writeJSON(toJSONDevice(st))
			}
			return env.printDevice(st)
		}),
	}
}

func (e Env) printDevice(st device.Status) error {
	if !st.Trusted {
		_, err := fmt.Fprintln(e.Out, "not trusted")
		return err
	}
	expires := "never"
	if !st.Expires.IsZero() {
		expires = st.Expires.Local().Format(time.DateTime)
	}
	if st.Stale {
		expires += " (passed, the next unlock asks the master password)"
	}
	_, err := fmt.Fprintf(e.Out, "trusted with the %s plugin\nsince          %s\nlast password  %s\npassword again %s\n",
		safetext.Line(st.Plugin), st.Since.Local().Format(time.DateTime), st.Confirmed.Local().Format(time.DateTime), expires)
	return err
}

func toJSONDevice(st device.Status) jsonDevice {
	return jsonDevice{Trusted: st.Trusted, Plugin: st.Plugin, Since: st.Since, Confirmed: st.Confirmed, Expires: st.Expires, Stale: st.Stale}
}
