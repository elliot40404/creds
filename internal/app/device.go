package app

import (
	"errors"
	"fmt"

	"filippo.io/age/plugin"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/device"
	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/vaultfiles"
)

func (s *Service) devices() *device.Store {
	return &device.Store{Path: s.Paths.Device(), MaxAge: s.CurrentConfig().Device.MaxAge, Now: s.Now}
}

func (s *Service) DeviceTrust(key string) error {
	pid, rec, err := device.ParseKey(key)
	if err != nil {
		return err
	}
	if err := s.requireVault(); err != nil {
		return err
	}
	id, err := s.askUnwrap(vaultfiles.PasswordFile, "Master password", asTyped)
	if err != nil {
		return err
	}
	if err := s.devices().Trust(id, pid, rec, s.pluginUI()); err != nil {
		return err
	}
	return s.sessions().Save(id.String())
}

func (s *Service) DeviceUntrust() error {
	return s.devices().Untrust()
}

func (s *Service) DeviceStatus() (device.Status, error) {
	return s.devices().Status()
}

func (s *Service) deviceIdentity() *crypto.Identity {
	meta, err := format.LoadMeta(s.vaultFile(vaultfiles.MetaFile))
	if err != nil {
		return nil
	}
	id, err := s.devices().Open(meta.Recipient, s.pluginUI())
	if err != nil && !errors.Is(err, device.ErrNotTrusted) {
		s.notice("device unlock skipped, " + err.Error())
	}
	return id
}

func (s *Service) confirmDevice() {
	if err := s.devices().Confirm(); err != nil {
		s.warn("could not update the device trust time: " + err.Error())
	}
}

func (s *Service) offerUntrust() error {
	st, err := s.devices().Status()
	if err != nil || !st.Trusted {
		return err
	}
	ok, err := s.Prompter.Confirm(fmt.Sprintf("This device still unlocks with the %s plugin. Untrust it too?", st.Plugin))
	if err != nil || !ok {
		return err
	}
	return s.devices().Untrust()
}

func (s *Service) notice(msg string) {
	if w, ok := s.Prompter.(warner); ok {
		w.Warn(msg)
		return
	}
	s.warn(msg)
}

func (s *Service) pluginUI() *plugin.ClientUI {
	label := func(name, text string) string { return safetext.Line(name + " plugin: " + text) }
	return &plugin.ClientUI{
		DisplayMessage: func(name, msg string) error {
			s.notice(label(name, msg))
			return nil
		},
		RequestValue: func(name, prompt string, secret bool) (string, error) {
			if secret {
				return s.Prompter.Password(label(name, prompt))
			}
			return s.Prompter.Input(label(name, prompt), "")
		},
		Confirm: func(name, prompt, _, _ string) (bool, error) {
			return s.Prompter.Confirm(label(name, prompt))
		},
		WaitTimer: func(name string) {
			s.notice(label(name, "waiting for touch or approval"))
		},
	}
}
