package process

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestStringRoundTrip(t *testing.T) {
	lines := []string{
		"npm ci",
		`go build -ldflags "-s -w" ./cmd/app`,
		`ssh host 'cd /app && export TAG=1.0.0'`,
		`echo "it's \"quoted\""`,
		`run ''`,
	}

	for _, line := range lines {
		command, parseError := Parse(line)
		if parseError != nil {
			t.Fatalf("Parse(%q): %v", line, parseError)
		}

		again, againError := Parse(command.String())
		if againError != nil {
			t.Fatalf("Parse(String(%q)): %v", line, againError)
		}

		if !reflect.DeepEqual(command, again) {
			t.Fatalf("round trip of %q: %#v != %#v", line, command, again)
		}
	}
}

func TestParseError(t *testing.T) {
	_, parseError := Parse("")
	if !errors.Is(parseError, ErrEmpty) {
		t.Fatalf("want ErrEmpty, got %v", parseError)
	}
}

func TestSystemOutput(t *testing.T) {
	system := System{}

	output, outputError := system.Output(context.Background(), Command{Name: "git", Arguments: []string{"--version"}})
	if outputError != nil || !strings.HasPrefix(output, "git version") {
		t.Fatalf("output = %q, %v", output, outputError)
	}
}

func TestSystemRunStreams(t *testing.T) {
	var stdout bytes.Buffer

	system := System{Stdout: &stdout, Stderr: &stdout}

	runError := system.Run(context.Background(), Command{Name: "git", Arguments: []string{"--version"}})
	if runError != nil || !strings.HasPrefix(stdout.String(), "git version") {
		t.Fatalf("stream = %q, %v", stdout.String(), runError)
	}
}

func TestSystemDirAndEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses env and pwd")
	}

	dir := t.TempDir()
	system := System{}

	environment, environmentError := system.Output(context.Background(), Command{Name: "env", Environment: []string{"SHIPO_TEST=42"}})
	if environmentError != nil || !strings.Contains(environment, "SHIPO_TEST=42") {
		t.Fatalf("environment not passed: %v", environmentError)
	}

	if !strings.Contains(environment, "PATH=") {
		t.Fatal("the rest of the environment must pass through")
	}

	working, workingError := system.Output(context.Background(), Command{Name: "pwd", Dir: dir})
	if workingError != nil || !strings.HasSuffix(working, dir[strings.LastIndex(dir, "/"):]) {
		t.Fatalf("pwd = %q, want %s", working, dir)
	}
}

func TestExitError(t *testing.T) {
	system := System{}
	command := Command{Name: "git", Arguments: []string{"no-such-command"}}

	_, outputError := system.Output(context.Background(), command)

	var exitError *ExitError
	if !errors.As(outputError, &exitError) {
		t.Fatalf("want *ExitError, got %T %v", outputError, outputError)
	}

	if exitError.Code == 0 || exitError.Stderr == "" {
		t.Fatalf("code = %d, stderr = %q", exitError.Code, exitError.Stderr)
	}

	if !strings.HasPrefix(exitError.Error(), "git no-such-command: ") || exitError.Unwrap() == nil {
		t.Fatalf("message = %q", exitError.Error())
	}
}

func TestMissingProgram(t *testing.T) {
	system := System{}

	runError := system.Run(context.Background(), Command{Name: "shipo-no-such-program"})

	var exitError *ExitError
	if !errors.As(runError, &exitError) || exitError.Code != -1 {
		t.Fatalf("want an ExitError with code -1, got %v", runError)
	}

	if system.Look("shipo-no-such-program") || !system.Look("git") {
		t.Fatal("Look")
	}
}

func TestCancel(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	runError := System{}.Run(cancelled, Command{Name: "git", Arguments: []string{"--version"}})
	if runError == nil {
		t.Fatal("a cancelled context must stop the command")
	}
}

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"":          "''",
		"plain":     "plain",
		"a b":       "'a b'",
		`say "hi"`:  `'say "hi"'`,
		"it's":      `"it's"`,
		`it's "x"`:  `"it's \"x\""`,
		`back\path`: `'back\path'`,
	}

	for argument, want := range cases {
		if got := quote(argument); got != want {
			t.Fatalf("quote(%q) = %s, want %s", argument, got, want)
		}
	}
}
