// Package ui prints shipo's output, in color on a terminal and as plain,
// stable text everywhere else, and asks the few questions the flow needs.
package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// The status marks at the start of a line.
const (
	MARK_DONE    = "✓"
	MARK_SKIP    = "-"
	MARK_FAIL    = "✗"
	MARK_RUNNING = "…"
)

// The widths that align the detail columns.
const (
	LABEL_WIDTH = 10
	ITEM_WIDTH  = 9
	FIELD_WIDTH = 8
	MENU_WIDTH  = 8
)

// ErrNoTerminal is returned when a prompt is needed but there is no terminal.
var ErrNoTerminal = errors.New("needs a terminal; run it yourself, or use the step commands")

// Terminal writes output and reads answers.
type Terminal struct {
	Out io.Writer
	Err io.Writer
	// IsInteractive is true when stdin is a terminal.
	IsInteractive bool
	// IsColor and IsErrorColor turn on colors for Out and Err.
	IsColor      bool
	IsErrorColor bool

	input *bufio.Reader
}

// NewTerminal returns a Terminal on the process's standard streams.
func NewTerminal() *Terminal {
	return &Terminal{
		Out:           os.Stdout,
		Err:           os.Stderr,
		IsInteractive: isTerminal(os.Stdin),
		IsColor:       isColorEnabled(os.Stdout),
		IsErrorColor:  isColorEnabled(os.Stderr),
		input:         bufio.NewReader(os.Stdin),
	}
}

// NewTestTerminal returns a plain Terminal that reads answers from input.
func NewTestTerminal(out io.Writer, input string, isInteractive bool) *Terminal {
	return &Terminal{Out: out, Err: out, IsInteractive: isInteractive, input: bufio.NewReader(strings.NewReader(input))}
}

func isTerminal(file *os.File) bool {
	return term.IsTerminal(int(file.Fd()))
}

func (terminal *Terminal) style(text string, styles ...string) string {
	return paint(terminal.IsColor, text, styles...)
}

func (terminal *Terminal) mark(isColor bool, mark string) string {
	return paint(isColor, mark, STYLE_BOLD, markStyles[mark])
}

// Line prints a step: a mark, a bold label, and its detail.
func (terminal *Terminal) Line(mark string, label string, detail string) {
	text := terminal.mark(terminal.IsColor, mark) + " " + terminal.style(pad(label, LABEL_WIDTH), STYLE_BOLD)

	if detail != "" {
		if mark == MARK_SKIP {
			detail = terminal.style(detail, STYLE_DIM)
		}

		text += " " + detail
	}

	_, _ = fmt.Fprintln(terminal.Out, strings.TrimRight(text, " "))
}

// Done prints a finished step.
func (terminal *Terminal) Done(label string, detail string) {
	terminal.Line(MARK_DONE, label, detail)
}

// Skip prints a skipped step.
func (terminal *Terminal) Skip(label string, detail string) {
	terminal.Line(MARK_SKIP, label, detail)
}

// Running prints a step that is in progress.
func (terminal *Terminal) Running(label string, detail string) {
	terminal.Line(MARK_RUNNING, label, detail)
}

// Fail prints a failed step to the error stream.
func (terminal *Terminal) Fail(label string, detail string) {
	text := terminal.mark(terminal.IsErrorColor, MARK_FAIL) + " " + paint(terminal.IsErrorColor, pad(label, LABEL_WIDTH), STYLE_BOLD) + " " + detail

	_, _ = fmt.Fprintln(terminal.Err, text)
}

// Error prints an error that ends the command.
func (terminal *Terminal) Error(problem error) {
	_, _ = fmt.Fprintln(terminal.Err, paint(terminal.IsErrorColor, "error:", STYLE_BOLD, STYLE_RED), problem)
}

// Print writes text as it is.
func (terminal *Terminal) Print(format string, arguments ...any) {
	_, _ = fmt.Fprintf(terminal.Out, format, arguments...)
}

// Heading prints a section title, such as "checks", with a blank line above
// it unless it's the first.
func (terminal *Terminal) Heading(title string, isFirst bool) {
	if !isFirst {
		_, _ = fmt.Fprintln(terminal.Out)
	}

	_, _ = fmt.Fprintln(terminal.Out, terminal.style(title, STYLE_BOLD))
}

// Item prints one indented line under a heading: a mark, a name, a detail.
func (terminal *Terminal) Item(mark string, name string, detail string) {
	text := "  " + terminal.mark(terminal.IsColor, mark) + " " + pad(name, ITEM_WIDTH) + " " + detail

	_, _ = fmt.Fprintln(terminal.Out, text)
}

// Note prints one indented, dimmed line under a heading.
func (terminal *Terminal) Note(text string) {
	_, _ = fmt.Fprintln(terminal.Out, "  "+terminal.mark(terminal.IsColor, MARK_SKIP)+" "+terminal.style(text, STYLE_DIM))
}

// Row prints one indented line under a heading, without a mark: a bold name
// and its detail.
func (terminal *Terminal) Row(name string, detail string) {
	_, _ = fmt.Fprintln(terminal.Out, "  "+terminal.style(pad(name, ITEM_WIDTH), STYLE_BOLD)+" "+detail)
}

// Field prints a name and its value, such as "version  1.4.0".
func (terminal *Terminal) Field(name string, value string) {
	_, _ = fmt.Fprintln(terminal.Out, terminal.style(pad(name, FIELD_WIDTH), STYLE_DIM)+" "+value)
}

// Block prints a bold label and one or more lines, aligned, without a mark.
func (terminal *Terminal) Block(label string, lines []string) {
	for index, line := range lines {
		name := ""
		if index == 0 {
			name = label
		}

		_, _ = fmt.Fprintln(terminal.Out, terminal.style(pad(name, LABEL_WIDTH), STYLE_BOLD)+" "+line)
	}
}

// Confirm asks a yes/no question. No is the default.
func (terminal *Terminal) Confirm(question string) (bool, error) {
	if !terminal.IsInteractive {
		return false, ErrNoTerminal
	}

	_, _ = fmt.Fprint(terminal.Out, terminal.style(question, STYLE_BOLD)+" "+terminal.style("[y/N]", STYLE_DIM)+" ")

	answer, readError := terminal.input.ReadString('\n')
	if readError != nil && answer == "" {
		return false, nil
	}

	answer = strings.ToLower(strings.TrimSpace(answer))

	return answer == "y" || answer == "yes", nil
}

// Choice is one menu entry.
type Choice struct {
	Label  string
	Detail string
}

// Menu shows numbered choices and returns the chosen index.
func (terminal *Terminal) Menu(title string, choices []Choice) (int, error) {
	if !terminal.IsInteractive {
		return 0, ErrNoTerminal
	}

	_, _ = fmt.Fprintf(terminal.Out, "%s\n\n", terminal.style(title, STYLE_BOLD))

	for index, choice := range choices {
		number := terminal.style(strconv.Itoa(index+1)+")", STYLE_DIM)
		arrow := terminal.style("→", STYLE_DIM)

		_, _ = fmt.Fprintf(terminal.Out, "  %s %s %s %s\n", number, pad(choice.Label, MENU_WIDTH), arrow, terminal.style(choice.Detail, STYLE_BOLD))
	}

	for {
		_, _ = fmt.Fprint(terminal.Out, terminal.style(">", STYLE_BOLD, STYLE_CYAN)+" ")

		answer, readError := terminal.input.ReadString('\n')
		if readError != nil && answer == "" {
			return 0, errors.New("cancelled")
		}

		number, parseError := strconv.Atoi(strings.TrimSpace(answer))
		if parseError == nil && number >= 1 && number <= len(choices) {
			_, _ = fmt.Fprintln(terminal.Out)

			return number - 1, nil
		}
	}
}
