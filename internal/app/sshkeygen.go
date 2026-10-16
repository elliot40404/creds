package app

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/elliot40404/creds/internal/sshdir"
)

var (
	ErrSSHKeygen     = errors.New("bad ssh keygen input")
	ErrSSHPassphrase = errors.New("ssh key passphrase is empty")
)

const maxSSHComment = 255

type SSHKeygen struct {
	Name       string
	Comment    string
	Passphrase bool
}

type SSHKeygenResult struct {
	Dir       string
	Rel       string
	Files     []SSHFileResult
	PublicKey string
}

func (s *Service) SSHKeygen(k SSHKeygen) (SSHKeygenResult, error) {
	if err := k.check(); err != nil {
		return SSHKeygenResult{}, err
	}
	dir, _, err := s.sshDirs()
	if err != nil {
		return SSHKeygenResult{}, err
	}
	if err := s.sshFree(dir, k.Name, k.Name+".pub"); err != nil {
		return SSHKeygenResult{}, err
	}
	files, err := s.newSSHKey(k)
	if err != nil {
		return SSHKeygenResult{}, err
	}
	adds := []sshAdd{{file: files[0], disk: true, vault: true}, {file: files[1], disk: true, vault: true}}
	out, err := s.addSSHFiles(dir, adds)
	if err != nil {
		return SSHKeygenResult{}, err
	}
	return SSHKeygenResult{Dir: dir, Rel: k.Name, Files: out, PublicKey: strings.TrimSpace(string(files[1].Data))}, nil
}

func (k SSHKeygen) check() error {
	if !sshNamePart.MatchString(k.Name) || k.Name == "config" || strings.HasSuffix(k.Name, ".pub") || sshdir.Ignored(k.Name) {
		return fmt.Errorf("%w: name %q, use letters, digits, . _ - and no .pub, like id_work", ErrSSHKeygen, k.Name)
	}
	if len(k.Comment) > maxSSHComment || !utf8.ValidString(k.Comment) || strings.ContainsFunc(k.Comment, unicode.IsControl) {
		return fmt.Errorf("%w: comment must be one line of text up to %d bytes", ErrSSHKeygen, maxSSHComment)
	}
	return nil
}

func (k SSHKeygen) comment() string {
	if k.Comment == "" {
		return k.Name
	}
	return k.Comment
}

func (s *Service) newSSHKey(k SSHKeygen) ([]sshdir.File, error) {
	var pass []byte
	if k.Passphrase {
		var err error
		if pass, err = Retry(s.Prompter, s.askSSHPassphrase, ErrSSHPassphrase, ErrPasswordMismatch); err != nil {
			return nil, err
		}
		defer clear(pass)
	}
	priv, pub, err := sshdir.NewKey(k.comment(), pass)
	if err != nil {
		return nil, err
	}
	return []sshdir.File{
		{Rel: k.Name, Kind: sshdir.KindPrivateKey, Data: priv},
		{Rel: k.Name + ".pub", Kind: sshdir.KindPublicKey, Data: pub},
	}, nil
}

func (s *Service) askSSHPassphrase() ([]byte, error) {
	pw, err := s.Prompter.Password("Key passphrase")
	if err != nil {
		return nil, err
	}
	if pw == "" {
		return nil, ErrSSHPassphrase
	}
	again, err := s.Prompter.Password("Repeat key passphrase")
	if err != nil {
		return nil, err
	}
	if again != pw {
		return nil, ErrPasswordMismatch
	}
	return []byte(pw), nil
}
