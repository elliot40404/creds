package app

import (
	"errors"
	"fmt"

	"github.com/elliot40404/creds/internal/vault"
)

var ErrNoField = errors.New("no such field")

func Lookup(e vault.Entry, key string) (string, error) {
	if p := BuiltinField(&e, key); p != nil {
		return *p, nil
	}
	if v, ok := e.Field(key); ok {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s", ErrNoField, key)
}
