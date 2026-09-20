package main

import (
	"os"

	"github.com/elliot40404/creds/internal/ui/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
