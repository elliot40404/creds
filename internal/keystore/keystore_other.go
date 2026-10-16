//go:build !windows && !linux && !darwin

package keystore

func platform() (Store, error) {
	return nil, ErrUnavailable
}
