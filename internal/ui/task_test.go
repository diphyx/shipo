package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

const CLEAR = "\r\x1b[2K"

func live(width int) (*Terminal, *bytes.Buffer) {
	var output bytes.Buffer

	terminal := NewTestTerminal(&output, "", false)
	terminal.IsLive = true
	terminal.size = func() int { return width }

	return terminal, &output
}

// screen replays the output the way a terminal shows it: a clear-line code
// starts the line over.
func screen(text string) string {
	var lines []string

	for _, line := range strings.Split(text, "\n") {
		parts := strings.Split(line, CLEAR)
		lines = append(lines, parts[len(parts)-1])
	}

	return strings.Join(lines, "\n")
}

func TestTaskShowsLatestLine(t *testing.T) {
	terminal, output := live(80)
	task := terminal.Start("build", "[1/1] npm ci")

	_, _ = fmt.Fprint(task, "added 1 package\nadded 312 packages\n")

	if got := screen(output.String()); got != "⠋ build      [1/1] npm ci  › added 312 packages" {
		t.Fatalf("live line = %q", got)
	}

	task.Done()

	if got := screen(output.String()); got != "✓ build      [1/1] npm ci\n" {
		t.Fatalf("done = %q", got)
	}
}

func TestTaskPartialAndCarriageReturn(t *testing.T) {
	terminal, output := live(80)
	task := terminal.Start("push", "origin/main")

	_, _ = fmt.Fprint(task, "Writing objects:  50%\rWriting objects: 100%")

	if got := screen(output.String()); got != "⠋ push       origin/main  › Writing objects: 100%" {
		t.Fatalf("live line = %q", got)
	}

	task.Done()

	if len(task.tail) != 1 || task.tail[0] != "Writing objects: 100%" {
		t.Fatalf("a line redrawn by \\r is kept once: %q", task.tail)
	}
}

func TestTaskStripsColorsAndCuts(t *testing.T) {
	terminal, output := live(40)
	task := terminal.Start("build", "make")

	_, _ = fmt.Fprint(task, "\x1b[32mcompiling a very long file name that does not fit\x1b[0m\n")

	got := screen(output.String())
	if strings.Contains(got, "\x1b[32m") || !strings.HasSuffix(got, "…") || len([]rune(got)) > 40 {
		t.Fatalf("live line = %q", got)
	}
}

func TestTaskNarrowTerminal(t *testing.T) {
	terminal, output := live(20)
	task := terminal.Start("command", "npm publish --access public")

	_, _ = fmt.Fprint(task, "anything\n")

	if strings.Contains(screen(output.String()), "›") {
		t.Fatal("with no room left, the latest line is not shown")
	}
}

func TestTaskFailShowsTail(t *testing.T) {
	terminal, output := live(80)
	task := terminal.Start("build", "[1/1] make")

	for index := 1; index <= 25; index++ {
		_, _ = fmt.Fprintf(task, "line %d\n", index)
	}

	_, _ = fmt.Fprint(task, "error: boom")
	task.Fail()

	got := screen(output.String())
	if !strings.HasPrefix(got, "✗ build      [1/1] make\n") {
		t.Fatalf("fail line = %q", got)
	}

	if strings.Contains(got, "line 6\n") || !strings.Contains(got, TAIL_PREFIX+"line 7\n") || !strings.HasSuffix(got, TAIL_PREFIX+"error: boom\n") {
		t.Fatalf("want the last %d lines, ending with the unfinished one:\n%s", TAIL_LINES, got)
	}
}

func TestTaskWithoutTerminalStreams(t *testing.T) {
	var output bytes.Buffer

	terminal := NewTestTerminal(&output, "", false)
	task := terminal.Start("build", "[1/1] make")

	_, _ = fmt.Fprint(task, "compiling\ndone\n")
	task.DoneAs("[1/1] make (2s)")

	want := "… build      [1/1] make\ncompiling\ndone\n✓ build      [1/1] make (2s)\n"
	if output.String() != want {
		t.Fatalf("got %q, want %q", output.String(), want)
	}
}

func TestTaskFailWithoutTerminal(t *testing.T) {
	var output bytes.Buffer

	terminal := NewTestTerminal(&output, "", false)
	task := terminal.Start("build", "make")

	_, _ = fmt.Fprint(task, "error: boom\n")
	task.Fail()

	if strings.Contains(output.String(), TAIL_PREFIX) {
		t.Fatal("without a terminal the output already streamed; no tail is repeated")
	}
}

func TestCut(t *testing.T) {
	if cut("short", 10) != "short" || cut("abcdefghij", 5) != "abcd…" {
		t.Fatal("cut")
	}
}

func TestTaskSpins(t *testing.T) {
	terminal, output := live(80)
	terminal.spinEvery = 5 * time.Millisecond

	task := terminal.Start("checks", "")
	time.Sleep(60 * time.Millisecond)
	task.Done()

	text := output.String()
	if !strings.Contains(text, "⠋") || !strings.Contains(text, "⠙") {
		t.Fatalf("the spinner must turn while the step waits: %q", text)
	}

	after := output.Len()
	time.Sleep(30 * time.Millisecond)

	if output.Len() != after {
		t.Fatal("the spinner must stop when the step ends")
	}
}
