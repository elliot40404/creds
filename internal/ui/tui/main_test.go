package tui

import (
	"os"
	"testing"

	"github.com/elliot40404/creds/internal/testutil"
)

var fixtureDir string

func TestMain(m *testing.M) {
	os.Exit(runWithFixtures(m))
}

func runWithFixtures(m *testing.M) int {
	dir, err := os.MkdirTemp("", "creds-tui-")
	if err != nil {
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	fixtureDir = dir
	return testutil.Run(m)
}
