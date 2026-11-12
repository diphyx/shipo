// Package testkit holds helpers for shipo's tests: a real git repository with
// a bare remote, and a fake runner for programs such as gh and npm.
package testkit

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/diphyx/shipo/internal/tool/process"
)

// Repo is a working repository with a bare "origin" remote.
type Repo struct {
	T      testing.TB
	Work   string
	Remote string
}

// Isolate keeps git from reading the user's config, and sets an identity.
func Isolate(t testing.TB) {
	t.Helper()

	environment := map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_SYSTEM":   os.DevNull,
		"GIT_AUTHOR_NAME":     "test",
		"GIT_AUTHOR_EMAIL":    "test@example.com",
		"GIT_COMMITTER_NAME":  "test",
		"GIT_COMMITTER_EMAIL": "test@example.com",
	}

	setter, isSetter := t.(interface{ Setenv(string, string) })
	if !isSetter {
		return
	}

	for name, value := range environment {
		setter.Setenv(name, value)
	}
}

// NewRepo creates a repository with files and a .shipo, commits them, and
// pushes main to the bare remote.
func NewRepo(t testing.TB, shipo string, files map[string]string) *Repo {
	t.Helper()
	Isolate(t)

	base := t.TempDir()
	repo := &Repo{T: t, Work: filepath.Join(base, "work"), Remote: filepath.Join(base, "remote.git")}

	Run(t, base, "git", "init", "--quiet", "--bare", "--initial-branch=main", repo.Remote)
	Run(t, base, "git", "init", "--quiet", "--initial-branch=main", repo.Work)
	repo.Git("remote", "add", "origin", repo.Remote)

	all := map[string]string{".shipo": shipo}
	for name, content := range files {
		all[name] = content
	}

	repo.Write(all)
	repo.Commit("init")
	repo.Git("push", "--quiet", "origin", "main")

	return repo
}

// Run runs a program in dir and fails the test if it fails.
func Run(t testing.TB, dir string, name string, arguments ...string) string {
	t.Helper()

	command := exec.Command(name, arguments...)
	command.Dir = dir

	output, runError := command.CombinedOutput()
	if runError != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), runError, output)
	}

	return strings.TrimSpace(string(output))
}

// Git runs git in the working repository.
func (repo *Repo) Git(arguments ...string) string {
	repo.T.Helper()

	return Run(repo.T, repo.Work, "git", arguments...)
}

// RemoteGit runs git in the bare remote.
func (repo *Repo) RemoteGit(arguments ...string) string {
	repo.T.Helper()

	return Run(repo.T, repo.Remote, "git", arguments...)
}

// Write writes files, relative to the working repository.
func (repo *Repo) Write(files map[string]string) {
	repo.T.Helper()

	for name, content := range files {
		path := filepath.Join(repo.Work, filepath.FromSlash(name))

		mkdirError := os.MkdirAll(filepath.Dir(path), 0o755)
		if mkdirError != nil {
			repo.T.Fatal(mkdirError)
		}

		writeError := os.WriteFile(path, []byte(content), 0o644)
		if writeError != nil {
			repo.T.Fatal(writeError)
		}
	}
}

// Read reads a file of the working repository.
func (repo *Repo) Read(name string) string {
	repo.T.Helper()

	data, readError := os.ReadFile(filepath.Join(repo.Work, filepath.FromSlash(name)))
	if readError != nil {
		repo.T.Fatal(readError)
	}

	return string(data)
}

// Commit stages everything and commits.
func (repo *Repo) Commit(message string) {
	repo.T.Helper()

	repo.Git("add", "--all")
	repo.Git("commit", "--quiet", "--allow-empty", "--message", message)
}

// Response is what a fake program returns.
type Response struct {
	Output string
	Error  error
}

// ErrFailed is a generic failure for fake programs.
var ErrFailed = errors.New("fake program failed")

// Runner passes some programs to the real system and fakes the rest.
type Runner struct {
	// Real lists programs that run for real, such as git.
	Real map[string]bool
	// Installed lists fake programs that Look finds.
	Installed map[string]bool
	// Handle answers a fake call. Nil means empty output and no error.
	Handle func(command process.Command) Response
	// Stream receives what real programs print.
	Stream *bytes.Buffer

	mutex sync.Mutex
	calls []process.Command
}

// NewRunner returns a Runner where git is real and the given programs are installed fakes.
func NewRunner(installed ...string) *Runner {
	runner := &Runner{Real: map[string]bool{"git": true}, Installed: map[string]bool{}, Stream: &bytes.Buffer{}}

	for _, name := range installed {
		runner.Installed[name] = true
	}

	return runner
}

func (runner *Runner) system() process.System {
	return process.System{Stdout: runner.Stream, Stderr: runner.Stream}
}

func (runner *Runner) fake(command process.Command) Response {
	runner.mutex.Lock()
	runner.calls = append(runner.calls, command)
	runner.mutex.Unlock()

	if runner.Handle == nil {
		return Response{}
	}

	return runner.Handle(command)
}

// Run runs a real program, or records and answers a fake one.
func (runner *Runner) Run(context context.Context, command process.Command) error {
	if runner.Real[command.Name] {
		return runner.system().Run(context, command)
	}

	return runner.fake(command).Error
}

// Output runs a real program, or records and answers a fake one.
func (runner *Runner) Output(context context.Context, command process.Command) (string, error) {
	if runner.Real[command.Name] {
		return runner.system().Output(context, command)
	}

	response := runner.fake(command)

	return response.Output, response.Error
}

// Look finds real programs on the PATH, and fakes in Installed.
func (runner *Runner) Look(name string) bool {
	if runner.Real[name] {
		return runner.system().Look(name)
	}

	return runner.Installed[name]
}

// Calls returns the fake calls, each as one line: "gh release view v1.0.0".
func (runner *Runner) Calls() []string {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()

	lines := make([]string, len(runner.calls))
	for index, command := range runner.calls {
		lines[index] = strings.TrimSpace(command.Name + " " + strings.Join(command.Arguments, " "))
	}

	return lines
}

// Called reports whether a fake call starts with prefix.
func (runner *Runner) Called(prefix string) bool {
	for _, line := range runner.Calls() {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}

	return false
}

// WithReplies builds a Handle that answers by the start of the call line.
func WithReplies(responses map[string]Response) func(process.Command) Response {
	prefixes := make([]string, 0, len(responses))
	for prefix := range responses {
		prefixes = append(prefixes, prefix)
	}

	sort.Slice(prefixes, func(left int, right int) bool {
		return len(prefixes[left]) > len(prefixes[right])
	})

	return func(command process.Command) Response {
		line := strings.TrimSpace(command.Name + " " + strings.Join(command.Arguments, " "))

		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) {
				return responses[prefix]
			}
		}

		return Response{}
	}
}
