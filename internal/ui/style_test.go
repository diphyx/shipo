package ui

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func colored(input string) (*Terminal, *bytes.Buffer) {
	var output bytes.Buffer

	terminal := NewTestTerminal(&output, input, true)
	terminal.IsColor = true
	terminal.IsErrorColor = true

	return terminal, &output
}

func TestPaint(t *testing.T) {
	if paint(false, "x", STYLE_BOLD) != "x" || paint(true, "", STYLE_BOLD) != "" || paint(true, "x") != "x" {
		t.Fatal("no color, no text, or no style must give the text as it is")
	}

	if got := paint(true, "ok", STYLE_BOLD, STYLE_GREEN); got != "\x1b[1;32mok\x1b[0m" {
		t.Fatalf("paint = %q", got)
	}
}

func TestPad(t *testing.T) {
	if pad("tag", 6) != "tag   " || pad("version", 3) != "version" || pad("…", 3) != "…  " {
		t.Fatal("pad counts characters, not bytes, and never cuts")
	}
}

func TestColoredLines(t *testing.T) {
	terminal, output := colored("")

	terminal.Done("tag", "v1.0.0")
	terminal.Skip("command", "skipped")
	terminal.Fail("push", "rejected")

	text := output.String()

	for _, want := range []string{
		"\x1b[1;32m✓\x1b[0m \x1b[1mtag       \x1b[0m v1.0.0\n",
		"\x1b[1;2m-\x1b[0m \x1b[1mcommand   \x1b[0m \x1b[2mskipped\x1b[0m\n",
		"\x1b[1;31m✗\x1b[0m \x1b[1mpush      \x1b[0m rejected\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
}

func TestColorKeepsColumns(t *testing.T) {
	plain := NewTestTerminal(&bytes.Buffer{}, "", false)
	terminal, output := colored("")

	terminal.Done("version", "package.json → 1.5.0")
	plain.Out = &bytes.Buffer{}
	plain.Done("version", "package.json → 1.5.0")

	stripped := strings.NewReplacer("\x1b[1;32m", "", "\x1b[1m", "", "\x1b[0m", "").Replace(output.String())
	if stripped != plain.Out.(*bytes.Buffer).String() {
		t.Fatalf("colors must not move the columns:\n%q\n%q", stripped, plain.Out)
	}
}

func TestColoredMenuAndConfirm(t *testing.T) {
	terminal, output := colored("1\ny\n")

	_, menuError := terminal.Menu(context.Background(), "app (1.0.0)", []Choice{{Label: "patch", Detail: "1.0.1"}})
	_, confirmError := terminal.Confirm(context.Background(), "Run?")

	if menuError != nil || confirmError != nil {
		t.Fatal(menuError, confirmError)
	}

	text := output.String()
	for _, want := range []string{"\x1b[1mapp (1.0.0)\x1b[0m", "\x1b[2m1)\x1b[0m patch    \x1b[2m→\x1b[0m \x1b[1m1.0.1\x1b[0m", "\x1b[1;36m>\x1b[0m ", "\x1b[1mRun?\x1b[0m \x1b[2m[y/N]\x1b[0m "} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
}

func TestSections(t *testing.T) {
	var output bytes.Buffer

	terminal := NewTestTerminal(&output, "", false)
	terminal.Heading("checks", true)
	terminal.Item(MARK_DONE, "tools", "git")
	terminal.Heading("release", false)
	terminal.Note("HEAD is not a release")
	terminal.Heading("plan", false)
	terminal.Row("push", "origin/main + v1.0.1")
	terminal.Field("version", "1.0.0")

	want := "checks\n  ✓ tools     git\n\nrelease\n  - HEAD is not a release\n\nplan\n  push      origin/main + v1.0.1\nversion  1.0.0\n"
	if output.String() != want {
		t.Fatalf("got:\n%q\nwant:\n%q", output.String(), want)
	}
}

func TestError(t *testing.T) {
	terminal, output := colored("")
	terminal.Error(errors.New("boom"))

	if output.String() != "\x1b[1;31merror:\x1b[0m boom\n" {
		t.Fatalf("got %q", output.String())
	}

	var plain bytes.Buffer

	NewTestTerminal(&plain, "", false).Error(errors.New("boom"))

	if plain.String() != "error: boom\n" {
		t.Fatalf("plain = %q", plain.String())
	}
}

func TestColorRules(t *testing.T) {
	file, createError := os.CreateTemp(t.TempDir(), "out")
	if createError != nil {
		t.Fatal(createError)
	}

	defer func() { _ = file.Close() }()

	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("TERM", "xterm-256color")

	if isColorEnabled(file) {
		t.Fatal("a file is not a terminal: no color")
	}

	t.Setenv("FORCE_COLOR", "1")

	if !isColorEnabled(file) {
		t.Fatal("FORCE_COLOR turns color on")
	}

	t.Setenv("NO_COLOR", "1")

	if isColorEnabled(file) {
		t.Fatal("NO_COLOR wins over FORCE_COLOR")
	}

	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("TERM", "dumb")

	if isColorEnabled(file) {
		t.Fatal("TERM=dumb means no color")
	}
}
