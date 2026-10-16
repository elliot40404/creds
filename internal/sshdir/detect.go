package sshdir

import (
	"bytes"
	"encoding/pem"
	"errors"
	"path"
	"strings"

	"golang.org/x/crypto/ssh"
)

var (
	puttyHeader  = []byte("PuTTY-User-Key-File-")
	opensshMagic = []byte("openssh-key-v1\x00")
)

func Ignored(name string) bool {
	return strings.HasPrefix(name, "known_hosts") || strings.HasPrefix(name, "authorized_keys")
}

func Classify(rel string, data []byte) Kind {
	switch {
	case Ignored(path.Base(rel)):
		return 0
	case isPrivateKey(data):
		return KindPrivateKey
	case strings.HasSuffix(rel, ".pub"):
		return KindPublicKey
	case rel == "config" || strings.HasPrefix(rel, "config.d/"):
		return KindConfig
	}
	return 0
}

func isPrivateKey(data []byte) bool {
	if bytes.HasPrefix(data, puttyHeader) || isOpenSSHKey(data) {
		return true
	}
	_, err := ssh.ParseRawPrivateKey(data)
	var missing *ssh.PassphraseMissingError
	return err == nil || errors.As(err, &missing)
}

func IsPublicKey(data []byte) bool {
	_, _, _, _, err := ssh.ParseAuthorizedKey(data)
	return err == nil
}

func isOpenSSHKey(data []byte) bool {
	block, _ := pem.Decode(data)
	return block != nil && block.Type == "OPENSSH PRIVATE KEY" && bytes.HasPrefix(block.Bytes, opensshMagic)
}
