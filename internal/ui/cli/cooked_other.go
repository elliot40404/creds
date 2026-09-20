//go:build !windows

package cli

import "os"

func cookInput(*os.File) {}
