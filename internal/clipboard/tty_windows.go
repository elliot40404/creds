package clipboard

import "os"

func OpenTTY() (*os.File, error) {
	return os.OpenFile("CONOUT$", os.O_WRONLY, 0)
}
