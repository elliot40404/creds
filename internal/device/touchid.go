package device

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/elliot40404/creds/internal/safetext"
)

const (
	touchIDPlugin  = "age-plugin-se"
	keygenTimeout  = time.Minute
	maxKeygenError = 1 << 10
)

var ErrNoTouchID = errors.New("device: --touchid needs macOS with a Secure Enclave")

var keygenArgs = []string{"keygen", "--access-control", "current-biometry", "--recipient-type", "se"}

func TouchIDKey(ctx context.Context) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", ErrNoTouchID
	}
	path, err := exec.LookPath(touchIDPlugin)
	if err != nil {
		return "", fmt.Errorf("%w: install it with brew install %s", ErrNoPlugin, touchIDPlugin)
	}
	ctx, cancel := context.WithTimeout(ctx, keygenTimeout)
	defer cancel()
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, path, keygenArgs...)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s keygen: %w: %s", touchIDPlugin, err, keygenError(stderr.String()+" "+out.String()))
	}
	return out.String(), nil
}

func keygenError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxKeygenError {
		s = s[:maxKeygenError]
	}
	return safetext.Line(s)
}
