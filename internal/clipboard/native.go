package clipboard

type Native interface {
	Read() (string, error)
	Write(value string) error
	Clear() error
}

type System struct{}

var _ Native = System{}
