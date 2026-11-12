// Package ui prints shipo's output, in color on a terminal and as plain,
// stable text everywhere else, and asks the few questions the flow needs.
package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

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

// ErrCancelled is returned when a prompt is stopped, by Ctrl-C or the end of input.
var ErrCancelled = errors.New("cancelled")

// Terminal writes output and reads answers.
type Terminal struct {
	Out io.Writer
	Err io.Writer
	// IsInteractive is true when stdin is a terminal.
	IsInteractive bool
	// IsColor and IsErrorColor turn on colors for Out and Err.
	IsColor      bool
	IsErrorColor bool
	// IsLive is true when Out is a terminal, so a task can redraw its line.
	IsLive bool

	input     io.Reader
	size      func() int
	spinEvery time.Duration
	// keyFile is the terminal a menu puts in raw mode to read arrow keys. Nil
	// means menus take a typed number.
	keyFile *os.File
	isKeys  bool

	readOnce sync.Once
	chunks   chan []byte
	pending  []byte
	isEnded  bool
}

// NewTerminal returns a Terminal on the process's standard streams.
func NewTerminal() *Terminal {
	return &Terminal{
		Out:           os.Stdout,
		Err:           os.Stderr,
		IsInteractive: isTerminal(os.Stdin),
		IsColor:       isColorEnabled(os.Stdout),
		IsErrorColor:  isColorEnabled(os.Stderr),
		IsLive:        isTerminal(os.Stdout) && os.Getenv("TERM") != "dumb",
		input:         os.Stdin,
		keyFile:       os.Stdin,
		isKeys:        isTerminal(os.Stdin) && isTerminal(os.Stdout) && os.Getenv("TERM") != "dumb",
		size:          func() int { return terminalWidth(os.Stdout) },
		spinEvery:     SPIN_EVERY,
	}
}

// NewTestTerminal returns a plain Terminal that reads answers from input.
func NewTestTerminal(out io.Writer, input string, isInteractive bool) *Terminal {
	return &Terminal{Out: out, Err: out, IsInteractive: isInteractive, input: strings.NewReader(input)}
}

func isTerminal(file *os.File) bool {
	return term.IsTerminal(int(file.Fd()))
}

func terminalWidth(file *os.File) int {
	width, _, sizeError := term.GetSize(int(file.Fd()))
	if sizeError != nil || width <= 0 {
		return DEFAULT_WIDTH
	}

	return width
}

// width is the terminal's width in columns.
func (terminal *Terminal) width() int {
	if terminal.size == nil {
		return DEFAULT_WIDTH
	}

	return terminal.size()
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
