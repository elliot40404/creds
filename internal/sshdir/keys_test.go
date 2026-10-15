package sshdir

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"golang.org/x/crypto/ssh"
)

func ed25519Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, k, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func rsaKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func ecdsaKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func openssh(t *testing.T, k crypto.PrivateKey) []byte {
	t.Helper()
	b, err := ssh.MarshalPrivateKey(k, "test")
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(b)
}

func opensshLocked(t *testing.T, k crypto.PrivateKey) []byte {
	t.Helper()
	b, err := ssh.MarshalPrivateKeyWithPassphrase(k, "test", []byte("pass"))
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(b)
}

func pkcs1(k *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
}

func pkcs8(t *testing.T, k crypto.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func publicKey(t *testing.T, k ed25519.PrivateKey) []byte {
	t.Helper()
	pub, err := ssh.NewPublicKey(k.Public())
	if err != nil {
		t.Fatal(err)
	}
	return ssh.MarshalAuthorizedKey(pub)
}

func unknownOpenSSH() []byte {
	body := append([]byte("openssh-key-v1\x00"), "sk-ssh-ed25519@openssh.com"...)
	return pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: body})
}

const putty = "PuTTY-User-Key-File-3: ssh-ed25519\nEncryption: none\nComment: test\n"

func legacyLocked() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: map[string]string{"Proc-Type": "4,ENCRYPTED", "DEK-Info": "AES-128-CBC,00112233445566778899AABBCCDDEEFF"},
		Bytes:   []byte("sealed"),
	})
}
