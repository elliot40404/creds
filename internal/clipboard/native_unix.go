//go:build !windows

package clipboard

func (System) Read() (string, error) {
	return systemToolSet().read()
}

func (System) Write(value string) error {
	return systemToolSet().write(value)
}

func (System) Clear() error {
	return systemToolSet().clear()
}
