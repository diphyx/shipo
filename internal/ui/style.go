package ui

import (
	"os"
	"strings"
)

// The ANSI styles shipo uses.
const (
	STYLE_BOLD  = "1"
	STYLE_DIM   = "2"
	STYLE_RED   = "31"
	STYLE_GREEN = "32"
	STYLE_CYAN  = "36"
)

// markStyles gives each status mark its color.
var markStyles = map[string]string{
	MARK_DONE:    STYLE_GREEN,
	MARK_FAIL:    STYLE_RED,
	MARK_RUNNING: STYLE_CYAN,
	MARK_SKIP:    STYLE_DIM,
}

// isColorEnabled reports whether a stream gets colors: a terminal, without
// NO_COLOR, and not TERM=dumb. FORCE_COLOR turns colors on anywhere.
func isColorEnabled(file *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}

	if os.Getenv("TERM") == "dumb" || !isTerminal(file) {
		return false
	}

	return enableColor(file)
}

// paint wraps text in ANSI styles, or returns it as it is when colors are off.
func paint(isColor bool, text string, styles ...string) string {
	if !isColor || text == "" || len(styles) == 0 {
		return text
	}

	return "\x1b[" + strings.Join(styles, ";") + "m" + text + "\x1b[0m"
}

// pad fills text with spaces up to width, before any style is added.
func pad(text string, width int) string {
	length := len([]rune(text))
	if length >= width {
		return text
	}

	return text + strings.Repeat(" ", width-length)
}
