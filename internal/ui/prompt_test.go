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

var bumps = []Choice{
	{Label: "patch", Detail: "1.1.1"},
	{Label: "minor", Detail: "1.2.0"},
	{Label: "major", Detail: "2.0.0"},
	{Label: "current", Detail: "1.1.0"},
}

func keys(input string) (*Terminal, *bytes.Buffer) {
	var output bytes.Buffer

	terminal := NewTestTerminal(&output, input, true)
	terminal.isKeys = true

	return terminal, &output
}

func TestMenuArrows(t *testing.T) {
	cases := map[string]int{
		"\r":                       0,
		KEY_DOWN + "\r":            1,
		KEY_DOWN + KEY_DOWN + "\n": 2,
		KEY_UP + "\r":              3,
		"jjk\r":                    1,
		"3":                        2,
		"x" + KEY_DOWN + "\r":      1,
		KEY_DOWN + KEY_DOWN + KEY_DOWN + KEY_DOWN + "\r": 0,
	}

	for input, want := range cases {
		terminal, _ := keys(input)

		index, menuError := terminal.Menu(context.Background(), "go (1.1.0)", bumps)
		if menuError != nil || index != want {
			t.Fatalf("%q: got %d, %v; want %d", input, index, menuError, want)
		}
	}
}

func TestMenuArrowsDraw(t *testing.T) {
	terminal, output := keys(KEY_DOWN + "\r")

	_, _ = terminal.Menu(context.Background(), "go (1.1.0)", bumps)

	text := output.String()

	if !strings.HasPrefix(text, CODE_HIDE+"go (1.1.0)\r\n\r\n"+CODE_CLEAR+MENU_CURSOR+" patch    → 1.1.1\r\n") {
		t.Fatalf("first draw:\n%q", text)
	}

	if !strings.Contains(text, CODE_CLEAR+"  patch    → 1.1.1\r\n"+CODE_CLEAR+MENU_CURSOR+" minor    → 1.2.0\r\n") {
		t.Fatalf("the cursor must move to minor:\n%q", text)
	}

	if !strings.HasSuffix(text, "\x1b[6A\r"+CODE_CLEAR_UP+CODE_SHOW) {
		t.Fatalf("the menu must clear itself and show the cursor again:\n%q", text)
	}
}

func TestMenuArrowsCancel(t *testing.T) {
	for _, input := range []string{"q", "\x03", "\x04", ""} {
		terminal, output := keys(input)

		_, menuError := terminal.Menu(context.Background(), "go (1.1.0)", bumps)
		if !errors.Is(menuError, ErrCancelled) {
			t.Fatalf("%q: want ErrCancelled, got %v", input, menuError)
		}

		if !strings.HasSuffix(output.String(), CODE_SHOW) {
			t.Fatalf("%q: the cursor must be shown again", input)
		}
	}
}

func TestMenuKeysThenLines(t *testing.T) {
	terminal, _ := keys(KEY_DOWN + "\ry\n")

	index, _ := terminal.Menu(context.Background(), "x", bumps)
	isYes, confirmError := terminal.Confirm(context.Background(), "Run?")

	if index != 1 || !isYes || confirmError != nil {
		t.Fatalf("a confirm after the menu reads the next line: %d %v %v", index, isYes, confirmError)
	}
}

func TestReadKeyUTF8(t *testing.T) {
	terminal, _ := keys("é→")

	first, _ := terminal.readKey(context.Background())
	second, _ := terminal.readKey(context.Background())

	if first != "é" || second != "→" {
		t.Fatalf("got %q %q", first, second)
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

func TestMenuFrom(t *testing.T) {
	terminal, output := keys(KEY_DOWN + "\r")

	index, menuError := terminal.MenuFrom(context.Background(), "target", []Choice{{Label: "."}, {Label: "packages/api"}, {Label: "packages/web"}}, 1)
	if menuError != nil || index != 2 {
		t.Fatalf("the cursor starts on the selected choice: %d %v", index, menuError)
	}

	if !strings.Contains(output.String(), MENU_CURSOR+" packages/api → ") {
		t.Fatalf("labels wider than MENU_WIDTH set the column:\n%q", output)
	}
}
