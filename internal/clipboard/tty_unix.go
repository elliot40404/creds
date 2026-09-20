//go:build !windows

package clipboard

import "os"

func OpenTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_WRONLY, 0)
}
