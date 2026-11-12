package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
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

		isYes, confirmError := NewTestTerminal(&output, input, true).Confirm("Run?")
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
	_, confirmError := NewTestTerminal(&bytes.Buffer{}, "y\n", false).Confirm("Run?")
	if !errors.Is(confirmError, ErrNoTerminal) {
		t.Fatalf("want ErrNoTerminal, got %v", confirmError)
	}
}

func TestMenu(t *testing.T) {
	choices := []Choice{{Label: "patch", Detail: "1.0.1"}, {Label: "minor", Detail: "1.1.0"}}

	var output bytes.Buffer

	index, menuError := NewTestTerminal(&output, "0\nabc\n3\n2\n", true).Menu("app (1.0.0)", choices)
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
	_, menuError := NewTestTerminal(&bytes.Buffer{}, "", true).Menu("x", []Choice{{Label: "a"}})
	if menuError == nil {
		t.Fatal("want an error when input ends")
	}

	_, terminalError := NewTestTerminal(&bytes.Buffer{}, "1\n", false).Menu("x", []Choice{{Label: "a"}})
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
