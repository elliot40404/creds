package main

import (
	"errors"
	"os"

	"filippo.io/age"
	"filippo.io/age/plugin"
)

const (
	modeEnv = "CREDS_TEST_PLUGIN"
	testPIN = "1234"
)

type identity struct {
	p  *plugin.Plugin
	id *age.X25519Identity
}

func main() {
	p, err := plugin.New("credstest")
	if err != nil {
		os.Exit(1)
	}
	p.HandleRecipient(func(data []byte) (age.Recipient, error) {
		return age.ParseX25519Recipient(string(data))
	})
	p.HandleIdentity(func(data []byte) (age.Identity, error) {
		id, err := age.ParseX25519Identity(string(data))
		return &identity{p: p, id: id}, err
	})
	os.Exit(p.Main())
}

func (i *identity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	if err := i.interact(); err != nil {
		return nil, err
	}
	return i.id.Unwrap(stanzas)
}

func (i *identity) interact() error {
	switch os.Getenv(modeEnv) {
	case "cancel":
		return errors.New("canceled by user")
	case "msg":
		return i.p.DisplayMessage("touch your key")
	case "pin":
		pin, err := i.p.RequestValue("PIN", true)
		if err != nil {
			return err
		}
		if pin != testPIN {
			return errors.New("wrong PIN")
		}
	}
	return nil
}
