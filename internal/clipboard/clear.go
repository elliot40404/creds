package clipboard

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const ClearCommand = "__clipclear"

var (
	ErrBadHash = errors.New("invalid clipboard hash")
	ErrBadMode = errors.New("invalid clipboard mode")
)

func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func readHash(r io.Reader) (string, error) {
	line, err := bufio.NewReader(io.LimitReader(r, 2*sha256.Size+1)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if b, err := hex.DecodeString(line); err != nil || len(b) != sha256.Size {
		return "", ErrBadHash
	}
	return line, nil
}

func clearIfUnchanged(n Native, hash string) (bool, error) {
	cur, err := n.Read()
	if err != nil {
		return false, err
	}
	if subtle.ConstantTimeCompare([]byte(Hash(cur)), []byte(hash)) != 1 {
		return false, nil
	}
	return true, n.Clear()
}

type ClearRun struct {
	In       io.Reader
	After    time.Duration
	Mode     Mode
	Native   Native
	Terminal Terminal
	Sleep    func(time.Duration)
}

func (c ClearRun) Run() error {
	hash, err := readHash(c.In)
	if err != nil {
		return err
	}
	c.Sleep(c.After)
	switch c.Mode {
	case ModeNative:
		_, err = clearIfUnchanged(c.Native, hash)
		return err
	case ModeOSC52:
		return c.Terminal.Write("")
	}
	return fmt.Errorf("%w: %v", ErrBadMode, c.Mode)
}
