package ui

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// The live line and the failure report.
const (
	TAIL_LINES    = 20
	DEFAULT_WIDTH = 80
	TAIL_PREFIX   = "  │ "
)

var escapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|\x1b\][^\x07]*\x07`)

// Task is a step that runs a program. On a terminal it shows one line that
// follows the program's latest output; elsewhere the output streams as is.
//
// A Task is an io.Writer: give it to process.Command.Output.
type Task struct {
	terminal *Terminal
	label    string
	detail   string

	mutex   sync.Mutex
	partial strings.Builder
	latest  string
	tail    []string
}

// Start shows a step as running and returns it, to follow its program.
func (terminal *Terminal) Start(label string, detail string) *Task {
	task := &Task{terminal: terminal, label: label, detail: detail}

	if terminal.IsLive {
		task.draw()
	} else {
		terminal.Running(label, detail)
	}

	return task
}

// Write takes the program's output. On a terminal only its latest line is
// shown; the last lines are kept for a failure report.
func (task *Task) Write(data []byte) (int, error) {
	task.mutex.Lock()
	defer task.mutex.Unlock()

	if !task.terminal.IsLive {
		return task.terminal.Out.Write(data)
	}

	for _, character := range string(data) {
		switch character {
		case '\n':
			{
				task.endLine(true)
			}
		case '\r':
			{
				task.endLine(false)
			}
		default:
			{
				task.partial.WriteRune(character)
			}
		}
	}

	task.draw()

	return len(data), nil
}

// endLine finishes the current line. A carriage return only redraws it, as
// progress bars do; a newline also keeps it for the failure report.
func (task *Task) endLine(isKept bool) {
	line := strings.TrimSpace(escapePattern.ReplaceAllString(task.partial.String(), ""))
	task.partial.Reset()

	if line == "" {
		return
	}

	task.latest = line

	if isKept {
		task.tail = append(task.tail, line)
		if len(task.tail) > TAIL_LINES {
			task.tail = task.tail[len(task.tail)-TAIL_LINES:]
		}
	}
}

// draw rewrites the live line: the running step, then the latest output.
func (task *Task) draw() {
	terminal := task.terminal
	head := terminal.mark(terminal.IsColor, MARK_RUNNING) + " " + terminal.style(pad(task.label, LABEL_WIDTH), STYLE_BOLD) + " " + task.detail
	visible := 2 + LABEL_WIDTH + 1 + len([]rune(task.detail))

	latest := strings.TrimSpace(escapePattern.ReplaceAllString(task.partial.String(), ""))
	if latest == "" {
		latest = task.latest
	}

	room := terminal.width() - visible - 5
	if latest != "" && room > 8 {
		head += "  " + terminal.style("› "+cut(latest, room), STYLE_DIM)
	}

	_, _ = fmt.Fprint(terminal.Out, "\r\x1b[2K"+head)
}

// Done replaces the live line with the finished step.
func (task *Task) Done() {
	task.DoneAs(task.detail)
}

// DoneAs finishes the step with another detail, such as a count of files.
func (task *Task) DoneAs(detail string) {
	task.finish()
	task.terminal.Done(task.label, detail)
}

// Fail replaces the live line with the failed step, followed on a terminal by
// the program's last lines, so the cause is in sight.
func (task *Task) Fail() {
	isLive := task.terminal.IsLive

	task.finish()
	task.terminal.Fail(task.label, task.detail)

	if !isLive {
		return
	}

	for _, line := range task.tail {
		_, _ = fmt.Fprintln(task.terminal.Err, paint(task.terminal.IsErrorColor, TAIL_PREFIX+line, STYLE_DIM))
	}
}

func (task *Task) finish() {
	task.mutex.Lock()
	defer task.mutex.Unlock()

	task.endLine(true)

	if task.terminal.IsLive {
		_, _ = fmt.Fprint(task.terminal.Out, "\r\x1b[2K")
	}
}

// cut shortens text to width characters, with an ellipsis.
func cut(text string, width int) string {
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}

	return string(runes[:width-1]) + "…"
}
