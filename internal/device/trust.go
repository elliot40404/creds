package device

import (
	"errors"
	"fmt"

	"filippo.io/age"
	"filippo.io/age/plugin"

	"github.com/elliot40404/creds/internal/crypto"
)

func (s *Store) Trust(id *crypto.Identity, pluginIdentity, recipient string, ui *plugin.ClientUI) error {
	pid, err := plugin.NewIdentity(pluginIdentity, ui)
	if err != nil {
		return fmt.Errorf("device: %w", err)
	}
	r, err := recipientFor(pid, recipient, ui)
	if err != nil {
		return err
	}
	sealed, err := crypto.SealIdentity(id, r)
	if err != nil {
		return pluginErr(pid.Name(), err)
	}
	if err := check(sealed, pid, id.Recipient().String()); err != nil {
		return err
	}
	now := s.now()
	return s.save(record{
		Plugin: pid.Name(), Identity: pluginIdentity, Recipient: recipient,
		Vault: id.Recipient().String(), Trusted: now, Confirmed: now, Sealed: sealed,
	})
}

func (s *Store) Open(vault string, ui *plugin.ClientUI) (*crypto.Identity, error) {
	r, err := s.load()
	if err != nil {
		return nil, err
	}
	if r.Vault != vault {
		return nil, s.discard(ErrVaultChanged)
	}
	if s.stale(r) {
		return nil, ErrStale
	}
	pid, err := plugin.NewIdentity(r.Identity, ui)
	if err != nil || pid.Name() != r.Plugin {
		return nil, s.discard(ErrCorrupt)
	}
	id, err := crypto.OpenIdentity(r.Sealed, pid)
	if err != nil {
		return nil, pluginErr(r.Plugin, err)
	}
	if id.Recipient().String() != vault {
		return nil, s.discard(ErrVaultChanged)
	}
	return id, nil
}

func recipientFor(pid *plugin.Identity, recipient string, ui *plugin.ClientUI) (age.Recipient, error) {
	if recipient == "" {
		return pid.Recipient(), nil
	}
	r, err := plugin.NewRecipient(recipient, ui)
	if err != nil {
		return nil, fmt.Errorf("device: %w", err)
	}
	if r.Name() != pid.Name() {
		return nil, fmt.Errorf("%w: recipient is for plugin %q, identity for %q", ErrMismatch, r.Name(), pid.Name())
	}
	return r, nil
}

func check(sealed []byte, pid *plugin.Identity, vault string) error {
	got, err := crypto.OpenIdentity(sealed, pid)
	if errors.Is(err, crypto.ErrWrongKey) {
		return ErrMismatch
	}
	if err != nil {
		return pluginErr(pid.Name(), err)
	}
	if got.Recipient().String() != vault {
		return ErrMismatch
	}
	return nil
}

func pluginErr(name string, err error) error {
	if _, ok := errors.AsType[*plugin.NotFoundError](err); ok {
		return fmt.Errorf("%w: install age-plugin-%s", ErrNoPlugin, name)
	}
	return err
}
