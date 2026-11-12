package ui

import (
	"bytes"
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

func TestNewTerminal(t *testing.T) {
	terminal := NewTerminal()
	if terminal.Out == nil || terminal.Err == nil || terminal.input == nil {
		t.Fatal("streams not set")
	}
}
