package keystore

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const methodDPAPI = "dpapi"

type dpapi struct{}

func platform() (Store, error) {
	return dpapi{}, nil
}

func (dpapi) Method() string {
	return methodDPAPI
}

func (dpapi) Seal(account string, plain []byte) ([]byte, error) {
	salt, err := sealInput(account, plain)
	if err != nil {
		return nil, err
	}
	var out windows.DataBlob
	in, ent := blob(plain), blob(salt)
	if err := windows.CryptProtectData(&in, nil, &ent, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("keystore: dpapi seal: %w", err)
	}
	return take(&out, false), nil
}

func (dpapi) Open(account string, sealed []byte) ([]byte, error) {
	salt, err := openInput(account, sealed)
	if err != nil {
		return nil, err
	}
	var out windows.DataBlob
	in, ent := blob(sealed), blob(salt)
	if err := windows.CryptUnprotectData(&in, nil, &ent, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpen, err)
	}
	return take(&out, true), nil
}

func (dpapi) Forget(account string) error {
	_, err := entropy(account)
	return err
}

func blob(b []byte) windows.DataBlob {
	n := len(b)
	if n == 0 || n > maxSealed {
		return windows.DataBlob{Data: new(byte)}
	}
	return windows.DataBlob{Size: uint32(n), Data: &b[0]}
}

func take(out *windows.DataBlob, secret bool) []byte {
	if out.Data == nil {
		return []byte{}
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) }()
	src := unsafe.Slice(out.Data, out.Size)
	b := make([]byte, len(src))
	copy(b, src)
	if secret {
		clear(src)
	}
	return b
}
