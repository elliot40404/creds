package clipboard

import (
	"encoding/binary"
	"errors"
	"runtime"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
	openTries     = 100
	openWait      = 10 * time.Millisecond
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpen        = user32.NewProc("OpenClipboard")
	procClose       = user32.NewProc("CloseClipboard")
	procEmpty       = user32.NewProc("EmptyClipboard")
	procGet         = user32.NewProc("GetClipboardData")
	procSet         = user32.NewProc("SetClipboardData")
	procAvailable   = user32.NewProc("IsClipboardFormatAvailable")
	procRegister    = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc = kernel32.NewProc("GlobalAlloc")
	procGlobalFree  = kernel32.NewProc("GlobalFree")
	procGlobalLock  = kernel32.NewProc("GlobalLock")
	procGlobalUnl   = kernel32.NewProc("GlobalUnlock")
	procGlobalSize  = kernel32.NewProc("GlobalSize")

	errOpen = errors.New("clipboard busy")

	privateFormats = []string{
		"ExcludeClipboardContentFromMonitorProcessing",
		"CanIncludeInClipboardHistory",
		"CanUploadToCloudClipboard",
	}
)

func (System) Read() (string, error) {
	var out string
	err := withClipboard(func() error {
		if r, _, _ := procAvailable.Call(cfUnicodeText); r == 0 {
			return nil
		}
		h, _, err := procGet.Call(cfUnicodeText)
		if h == 0 {
			return err
		}
		out, err = readGlobal(h)
		return err
	})
	return out, err
}

func (System) Write(value string) error {
	data, err := windows.UTF16FromString(value)
	if err != nil {
		return err
	}
	return withClipboard(func() error {
		if r, _, err := procEmpty.Call(); r == 0 {
			return err
		}
		for _, name := range privateFormats {
			if err := setPrivate(name); err != nil {
				return err
			}
		}
		return setGlobal(cfUnicodeText, u16Bytes(data))
	})
}

func (System) Clear() error {
	return withClipboard(func() error {
		if r, _, err := procEmpty.Call(); r == 0 {
			return err
		}
		return nil
	})
}

func withClipboard(fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	err := fn()
	if r, _, cerr := procClose.Call(); r == 0 && err == nil {
		err = cerr
	}
	return err
}

func openClipboard() error {
	for range openTries {
		if r, _, _ := procOpen.Call(0); r != 0 {
			return nil
		}
		time.Sleep(openWait)
	}
	return errOpen
}

func setPrivate(name string) error {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	f, _, err := procRegister.Call(uintptr(unsafe.Pointer(p)))
	if f == 0 {
		return err
	}
	return setGlobal(f, make([]byte, 4))
}

func setGlobal(format uintptr, data []byte) error {
	h, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if h == 0 {
		return err
	}
	if err := copyToGlobal(h, data); err != nil {
		_, _, _ = procGlobalFree.Call(h)
		return err
	}
	if r, _, err := procSet.Call(format, h); r == 0 {
		_, _, _ = procGlobalFree.Call(h)
		return err
	}
	return nil
}

func copyToGlobal(h uintptr, data []byte) error {
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		return err
	}
	copy(unsafe.Slice((*byte)(ptr(p)), len(data)), data)
	_, _, _ = procGlobalUnl.Call(h)
	return nil
}

func readGlobal(h uintptr) (string, error) {
	size, _, _ := procGlobalSize.Call(h)
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		return "", err
	}
	defer func() { _, _, _ = procGlobalUnl.Call(h) }()
	units := unsafe.Slice((*uint16)(ptr(p)), size/2)
	for i, c := range units {
		if c == 0 {
			units = units[:i]
			break
		}
	}
	return string(utf16.Decode(units)), nil
}

func u16Bytes(data []uint16) []byte {
	out := make([]byte, 0, 2*len(data))
	for _, c := range data {
		out = binary.LittleEndian.AppendUint16(out, c)
	}
	return out
}

func ptr(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}
