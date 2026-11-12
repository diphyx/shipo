package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/ui"
)

const PACKAGE_JSON = "{\n  \"name\": \"app\",\n  \"version\": \"1.0.0\"\n}\n"

const DISPATCH = "on:\n  workflow_dispatch:\n"

type session struct {
	repo   *testkit.Repo
	runner *testkit.Runner
	output *bytes.Buffer
}

func newSession(t *testing.T, shipo string, files map[string]string) *session {
	t.Helper()

	if files == nil {
		files = map[string]string{"package.json": PACKAGE_JSON, ".github/workflows/build.yml": DISPATCH}
	}

	return &session{repo: testkit.NewRepo(t, shipo, files), runner: testkit.NewRunner("gh", "npm"), output: &bytes.Buffer{}}
}

func (session *session) app(input string, isInteractive bool) *App {
	return &App{
		UI:      ui.NewTestTerminal(session.output, input, isInteractive),
		Runner:  session.runner,
		Dir:     session.repo.Work,
		Version: "9.9.9",
	}
}

func (session *session) run(t *testing.T, input string, isInteractive bool, arguments ...string) error {
	t.Helper()

	return session.app(input, isInteractive).Dispatch(context.Background(), arguments)
}

func TestRunExitCodes(t *testing.T) {
	stdout := os.Stdout

	devNull, openError := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if openError != nil {
		t.Fatal(openError)
	}

	os.Stdout, os.Stderr = devNull, devNull
	defer func() { os.Stdout, os.Stderr = stdout, stdout }()

	if code := Run([]string{"--version"}, "1.0.0"); code != 0 {
		t.Fatalf("--version exit code = %d", code)
	}

	if code := Run([]string{"no-such-command"}, "1.0.0"); code != 1 {
		t.Fatalf("unknown command exit code = %d", code)
	}
}

func TestDispatchBasics(t *testing.T) {
	current := newSession(t, "", nil)

	for _, name := range []string{"--version", "-v"} {
		current.output.Reset()

		if current.run(t, "", false, name) != nil || current.output.String() != "9.9.9\n" {
			t.Fatalf("%s printed %q", name, current.output)
		}
	}

	for _, name := range []string{"--help", "-h", "help"} {
		current.output.Reset()

		if current.run(t, "", false, name) != nil || !strings.HasPrefix(current.output.String(), "shipo cuts a release") {
			t.Fatalf("%s printed %q", name, current.output)
		}
	}
}

func TestDispatchUsageErrors(t *testing.T) {
	current := newSession(t, "", nil)

	cases := map[string][]string{
		"unknown command":       {"deploy"},
		"bump without level":    {"bump"},
		"bump with two":         {"bump", "patch", "minor"},
		"bump ask":              {"bump", "ask"},
		"bump unknown level":    {"bump", "huge"},
		"workflow bad flag":     {"workflow", "--wait"},
		"extra argument":        {"version", "now"},
		"flow extra argument":   {"patch", "now"},
		"step extra argument":   {"push", "now"},
		"skill three arguments": {"skill", "claude", "project", "x"},
	}

	for name, arguments := range cases {
		t.Run(name, func(t *testing.T) {
			if current.run(t, "", false, arguments...) == nil {
				t.Fatalf("want an error for %v", arguments)
			}
		})
	}
}
