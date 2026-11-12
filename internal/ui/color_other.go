//go:build !windows

package ui

import (
	"os"
)

// enableColor needs no setup outside Windows: terminals read ANSI styles.
func enableColor(_ *os.File) bool {
	return true
}
