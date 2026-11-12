package ui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestLines(t *testing.T) {
	var output bytes.Buffer

	terminal := NewTestTerminal(&output, "", false)
	terminal.Done("tag", "v1.0.0")
	terminal.Skip("command", "skipped")
	terminal.Running("build", "[1/2] make")
	terminal.Fail("push", "rejected")
	terminal.Line(MARK_DONE, "alone", "")
	terminal.Block("command", []string{"npm publish", "ssh host deploy"})
	terminal.Print("plain %d\n", 7)

	want := "" +
		"✓ tag        v1.0.0\n" +
		"- command    skipped\n" +
		"… build      [1/2] make\n" +
		"✗ push       rejected\n" +
		"✓ alone\n" +
		"command    npm publish\n" +
		"           ssh host deploy\n" +
		"plain 7\n"

	if output.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestConfirm(t *testing.T) {
	cases := map[string]bool{
		"y\n":   true,
		"Y\n":   true,
		"yes\n": true,
		" y \n": true,
		"n\n":   false,
		"\n":    false,
		"no\n":  false,
		"x\n":   false,
		"":      false,
	}

	for input, want := range cases {
		var output bytes.Buffer

		isYes, confirmError := NewTestTerminal(&output, input, true).Confirm(context.Background(), "Run?")
		if confirmError != nil {
			t.Fatalf("%q: %v", input, confirmError)
		}

		if isYes != want {
			t.Fatalf("%q: got %v, want %v", input, isYes, want)
		}

		if output.String() != "Run? [y/N] " {
			t.Fatalf("prompt = %q", output.String())
		}
	}
}

func TestConfirmNeedsTerminal(t *testing.T) {
	_, confirmError := NewTestTerminal(&bytes.Buffer{}, "y\n", false).Confirm(context.Background(), "Run?")
	if !errors.Is(confirmError, ErrNoTerminal) {
		t.Fatalf("want ErrNoTerminal, got %v", confirmError)
	}
}

func TestMenu(t *testing.T) {
	choices := []Choice{{Label: "patch", Detail: "1.0.1"}, {Label: "minor", Detail: "1.1.0"}}

	var output bytes.Buffer

	index, menuError := NewTestTerminal(&output, "0\nabc\n3\n2\n", true).Menu(context.Background(), "app (1.0.0)", choices)
	if menuError != nil || index != 1 {
		t.Fatalf("index = %d, %v", index, menuError)
	}

	text := output.String()
	if !strings.HasPrefix(text, "app (1.0.0)\n\n  1) patch    → 1.0.1\n  2) minor    → 1.1.0\n") {
		t.Fatalf("menu:\n%s", text)
	}

	if strings.Count(text, "> ") != 4 {
		t.Fatalf("want 4 prompts for 3 bad answers and 1 good, got:\n%s", text)
	}
}

func TestMenuEnd(t *testing.T) {
	_, menuError := NewTestTerminal(&bytes.Buffer{}, "", true).Menu(context.Background(), "x", []Choice{{Label: "a"}})
	if menuError == nil {
		t.Fatal("want an error when input ends")
	}

	_, terminalError := NewTestTerminal(&bytes.Buffer{}, "1\n", false).Menu(context.Background(), "x", []Choice{{Label: "a"}})
	if !errors.Is(terminalError, ErrNoTerminal) {
		t.Fatalf("want ErrNoTerminal, got %v", terminalError)
	}
}

func TestNewTerminal(t *testing.T) {
	terminal := NewTerminal()
	if terminal.Out == nil || terminal.Err == nil || terminal.input == nil {
		t.Fatal("streams not set")
	}
}

func TestPromptsStopOnCtrlC(t *testing.T) {
	reader, writer := io.Pipe()
	defer func() { _ = writer.Close() }()

	terminal := &Terminal{Out: &bytes.Buffer{}, IsInteractive: true, input: bufio.NewReader(reader)}

	for name, prompt := range map[string]func(context.Context) error{
		"menu": func(cancelled context.Context) error {
			_, menuError := terminal.Menu(cancelled, "x", []Choice{{Label: "a"}})

			return menuError
		},
		"confirm": func(cancelled context.Context) error {
			_, confirmError := terminal.Confirm(cancelled, "Run?")

			return confirmError
		},
	} {
		cancelled, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)

		go func() { done <- prompt(cancelled) }()

		cancel()

		select {
		case promptError := <-done:
			{
				if !errors.Is(promptError, ErrCancelled) {
					t.Fatalf("%s: want ErrCancelled, got %v", name, promptError)
				}
			}
		case <-time.After(2 * time.Second):
			{
				t.Fatalf("%s still waits for input after Ctrl-C", name)
			}
		}
	}
}
