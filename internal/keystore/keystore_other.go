//go:build !windows && !linux

package keystore

func platform() (Store, error) {
	return nil, ErrUnavailable
}
