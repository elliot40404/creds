package app

import "errors"

const MaxTries = 3

type warner interface {
	Warn(msg string)
}

func Retry[T any](p Prompter, ask func() (T, error), retryable ...error) (T, error) {
	var zero T
	for try := 1; ; try++ {
		v, err := ask()
		if err == nil {
			return v, nil
		}
		if try == MaxTries || !isAny(err, retryable) {
			return zero, err
		}
		Warn(p, err.Error()+", try again")
	}
}

func Warn(p Prompter, msg string) {
	if w, ok := p.(warner); ok {
		w.Warn(msg)
	}
}

func isAny(err error, targets []error) bool {
	for _, t := range targets {
		if errors.Is(err, t) {
			return true
		}
	}
	return false
}
