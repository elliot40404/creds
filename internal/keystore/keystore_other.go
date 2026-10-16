//go:build !windows

package keystore

func platform() (Store, error) {
	return nil, ErrUnavailable
}
