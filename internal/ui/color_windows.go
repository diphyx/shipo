//go:build windows

package ui

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableColor turns on ANSI styles in the Windows console. If the console
// can't, shipo prints plain text.
func enableColor(file *os.File) bool {
	handle := windows.Handle(file.Fd())

	var mode uint32

	modeError := windows.GetConsoleMode(handle, &mode)
	if modeError != nil {
		return false
	}

	setError := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)

	return setError == nil
}
