package ui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// The terminal codes the arrow-key menu uses.
const (
	KEY_UP        = "\x1b[A"
	KEY_DOWN      = "\x1b[B"
	CODE_HIDE     = "\x1b[?25l"
	CODE_SHOW     = "\x1b[?25h"
	CODE_CLEAR    = "\r\x1b[2K"
	CODE_CLEAR_UP = "\x1b[J"
	MENU_CURSOR   = "❯"
)

// Confirm asks a yes/no question. No is the default, and so is the end of input.
func (terminal *Terminal) Confirm(context context.Context, question string) (bool, error) {
	if !terminal.IsInteractive {
		return false, ErrNoTerminal
	}

	_, _ = fmt.Fprint(terminal.Out, terminal.style(question, STYLE_BOLD)+" "+terminal.style("[y/N]", STYLE_DIM)+" ")

	answer, readError := terminal.readLine(context)
	if readError != nil {
		if context.Err() != nil {
			return false, readError
		}

		return false, nil
	}

	answer = strings.ToLower(answer)

	return answer == "y" || answer == "yes", nil
}

// Choice is one menu entry.
type Choice struct {
	Label  string
	Detail string
}

// Menu shows choices and returns the chosen index. On a terminal it is moved
// with the arrow keys; elsewhere a number is typed.
func (terminal *Terminal) Menu(context context.Context, title string, choices []Choice) (int, error) {
	return terminal.MenuFrom(context, title, choices, 0)
}

// MenuFrom is Menu with the cursor first on the selected choice.
func (terminal *Terminal) MenuFrom(context context.Context, title string, choices []Choice, selected int) (int, error) {
	if !terminal.IsInteractive {
		return 0, ErrNoTerminal
	}

	if selected < 0 || selected >= len(choices) {
		selected = 0
	}

	if terminal.isKeys {
		index, keysError := terminal.menuKeys(context, title, choices, selected)
		if !errors.Is(keysError, errNoRawMode) {
			return index, keysError
		}
	}

	return terminal.menuNumbers(context, title, choices)
}

// menuNumbers shows numbered choices and reads the chosen number.
func (terminal *Terminal) menuNumbers(context context.Context, title string, choices []Choice) (int, error) {
	_, _ = fmt.Fprintf(terminal.Out, "%s\n\n", terminal.style(title, STYLE_BOLD))

	width := menuWidth(choices)

	for index, choice := range choices {
		number := terminal.style(strconv.Itoa(index+1)+")", STYLE_DIM)
		arrow := terminal.style("→", STYLE_DIM)

		_, _ = fmt.Fprintf(terminal.Out, "  %s %s %s %s\n", number, pad(choice.Label, width), arrow, terminal.style(choice.Detail, STYLE_BOLD))
	}

	for {
		_, _ = fmt.Fprint(terminal.Out, terminal.style(">", STYLE_BOLD, STYLE_CYAN)+" ")

		answer, readError := terminal.readLine(context)
		if readError != nil {
			return 0, readError
		}

		number, parseError := strconv.Atoi(strings.TrimSpace(answer))
		if parseError == nil && number >= 1 && number <= len(choices) {
			_, _ = fmt.Fprintln(terminal.Out)

			return number - 1, nil
		}
	}
}

// errNoRawMode means the terminal can't read single keys; the menu falls back
// to a typed number.
var errNoRawMode = errors.New("no raw mode")

// menuKeys shows choices with a cursor moved by the arrow keys, and clears
// itself once a choice is made.
func (terminal *Terminal) menuKeys(context context.Context, title string, choices []Choice, selected int) (int, error) {
	if terminal.keyFile != nil {
		state, rawError := term.MakeRaw(int(terminal.keyFile.Fd()))
		if rawError != nil {
			return 0, errNoRawMode
		}

		defer func() { _ = term.Restore(int(terminal.keyFile.Fd()), state) }()
	}

	_, _ = fmt.Fprint(terminal.Out, CODE_HIDE+terminal.style(title, STYLE_BOLD)+"\r\n\r\n")
	terminal.drawChoices(choices, selected)

	for {
		key, keyError := terminal.readKey(context)
		if keyError != nil {
			terminal.endMenu(len(choices))

			return 0, keyError
		}

		switch key {
		case KEY_UP, "k":
			{
				selected = (selected + len(choices) - 1) % len(choices)
			}
		case KEY_DOWN, "j":
			{
				selected = (selected + 1) % len(choices)
			}
		case "\r", "\n":
			{
				terminal.endMenu(len(choices))

				return selected, nil
			}
		case "q", "\x03", "\x04":
			{
				terminal.endMenu(len(choices))

				return 0, ErrCancelled
			}
		default:
			{
				if len(key) == 1 && key[0] >= '1' && int(key[0]-'0') <= len(choices) {
					terminal.endMenu(len(choices))

					return int(key[0] - '1'), nil
				}

				continue
			}
		}

		_, _ = fmt.Fprintf(terminal.Out, "\x1b[%dA", len(choices))
		terminal.drawChoices(choices, selected)
	}
}

// drawChoices writes one line per choice, with the cursor on the selected one.
func (terminal *Terminal) drawChoices(choices []Choice, selected int) {
	var builder strings.Builder

	width := menuWidth(choices)

	for index, choice := range choices {
		builder.WriteString(CODE_CLEAR)

		if index == selected {
			builder.WriteString(terminal.style(MENU_CURSOR, STYLE_BOLD, STYLE_CYAN) + " " + terminal.style(pad(choice.Label, width), STYLE_BOLD))
			builder.WriteString(" " + terminal.style("→", STYLE_DIM) + " " + terminal.style(choice.Detail, STYLE_BOLD))
		} else {
			builder.WriteString("  " + terminal.style(pad(choice.Label, width)+" → "+choice.Detail, STYLE_DIM))
		}

		builder.WriteString("\r\n")
	}

	_, _ = fmt.Fprint(terminal.Out, builder.String())
}

// endMenu clears the title, the blank line and the choices, and shows the
// cursor again: the chosen step prints its own line next.
func (terminal *Terminal) endMenu(count int) {
	_, _ = fmt.Fprintf(terminal.Out, "\x1b[%dA\r"+CODE_CLEAR_UP+CODE_SHOW, count+2)
}

// menuWidth is the label column: MENU_WIDTH, or the longest label.
func menuWidth(choices []Choice) int {
	width := MENU_WIDTH

	for _, choice := range choices {
		width = max(width, len([]rune(choice.Label)))
	}

	return width
}
