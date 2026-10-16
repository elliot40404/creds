package sshdir

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
)

func NewKey(comment string, passphrase []byte) (private, public []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	defer clear(priv)
	block, err := marshalKey(priv, comment, passphrase)
	if err != nil {
		return nil, nil, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(block), authorizedLine(sshPub, comment), nil
}

func marshalKey(priv ed25519.PrivateKey, comment string, passphrase []byte) (*pem.Block, error) {
	if len(passphrase) == 0 {
		return ssh.MarshalPrivateKey(priv, comment)
	}
	return ssh.MarshalPrivateKeyWithPassphrase(priv, comment, passphrase)
}

func authorizedLine(pub ssh.PublicKey, comment string) []byte {
	line := bytes.TrimSuffix(ssh.MarshalAuthorizedKey(pub), []byte("\n"))
	if comment != "" {
		line = append(append(line, ' '), comment...)
	}
	return append(line, '\n')
}
