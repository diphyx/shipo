package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Command is one program to spawn.
type Command struct {
	// Name is the program, looked up on the PATH.
	Name string
	// Arguments follow the program name.
	Arguments []string
	// Dir is the working directory. Empty means the current one.
	Dir string
	// Environment adds NAME=value entries on top of the current environment.
	Environment []string
	// Output receives the program's stdout and stderr while it runs. Nil means
	// the runner's own streams.
	Output io.Writer
}

// String returns the command as it would be typed.
func (command Command) String() string {
	parts := make([]string, 0, len(command.Arguments)+1)
	parts = append(parts, quote(command.Name))

	for _, argument := range command.Arguments {
		parts = append(parts, quote(argument))
	}

	return strings.Join(parts, " ")
}

// Parse splits a command line into a Command.
func Parse(line string) (Command, error) {
	arguments, splitError := Split(line)
	if splitError != nil {
		return Command{}, splitError
	}

	return Command{Name: arguments[0], Arguments: arguments[1:]}, nil
}

// Runner spawns commands. It is an interface so callers can be tested
// without real programs.
type Runner interface {
	// Run spawns the command and streams its output.
	Run(context context.Context, command Command) error
	// Output spawns the command and returns its standard output, trimmed.
	Output(context context.Context, command Command) (string, error)
	// Look reports whether a program is on the PATH.
	Look(name string) bool
}

// System is the Runner that spawns real programs.
type System struct {
	// Stdout and Stderr receive streamed output. Nil means os.Stdout and os.Stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// Run spawns the command, streaming its output to command.Output, or to
// Stdout and Stderr when that is nil.
func (system System) Run(context context.Context, command Command) error {
	process := system.build(context, command)
	process.Stdout = orDefault(system.Stdout, os.Stdout)
	process.Stderr = orDefault(system.Stderr, os.Stderr)

	if command.Output != nil {
		process.Stdout = command.Output
		process.Stderr = command.Output
	}

	return wrap(command, process.Run(), "")
}

// Output spawns the command and returns its trimmed standard output.
func (system System) Output(context context.Context, command Command) (string, error) {
	var stdout, stderr bytes.Buffer

	process := system.build(context, command)
	process.Stdout = &stdout
	process.Stderr = &stderr

	runError := process.Run()

	return strings.TrimSpace(stdout.String()), wrap(command, runError, strings.TrimSpace(stderr.String()))
}

// Look reports whether a program is on the PATH.
func (system System) Look(name string) bool {
	_, lookError := exec.LookPath(name)

	return lookError == nil
}

func (system System) build(context context.Context, command Command) *exec.Cmd {
	process := exec.CommandContext(context, command.Name, command.Arguments...)
	process.Dir = command.Dir
	process.Stdin = nil

	if len(command.Environment) > 0 {
		process.Env = append(os.Environ(), command.Environment...)
	}

	return process
}

// ExitError reports a command that failed.
type ExitError struct {
	Command Command
	Code    int
	Stderr  string
	Cause   error
}

// Error describes the failed command.
func (exitError *ExitError) Error() string {
	message := fmt.Sprintf("%s: %v", exitError.Command.String(), exitError.Cause)

	if exitError.Stderr != "" {
		message += ": " + exitError.Stderr
	}

	return message
}

// Unwrap returns the underlying error.
func (exitError *ExitError) Unwrap() error {
	return exitError.Cause
}

func wrap(command Command, runError error, stderr string) error {
	if runError == nil {
		return nil
	}

	code := -1

	var processError *exec.ExitError
	if errors.As(runError, &processError) {
		code = processError.ExitCode()
	}

	return &ExitError{Command: command, Code: code, Stderr: stderr, Cause: runError}
}

func orDefault(writer io.Writer, fallback io.Writer) io.Writer {
	if writer == nil {
		return fallback
	}

	return writer
}

func quote(argument string) string {
	if argument == "" {
		return `''`
	}

	if !strings.ContainsAny(argument, " \t\n'\"\\") {
		return argument
	}

	if !strings.Contains(argument, "'") {
		return "'" + argument + "'"
	}

	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(argument)

	return `"` + escaped + `"`
}
