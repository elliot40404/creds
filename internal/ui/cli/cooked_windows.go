package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

const cookedInput = windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT

func cookInput(f *os.File) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil || mode&cookedInput == cookedInput {
		return
	}
	_ = windows.SetConsoleMode(h, mode|cookedInput)
}
