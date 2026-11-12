// Package github wraps the gh CLI for releases and workflow runs.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/diphyx/shipo/internal/tool/process"
)

// PROGRAM is the GitHub CLI executable.
const PROGRAM = "gh"

// GitHub runs gh commands in one folder.
type GitHub struct {
	Runner process.Runner
	Dir    string
}

// NewGitHub returns a GitHub that runs in dir.
func NewGitHub(runner process.Runner, dir string) *GitHub {
	return &GitHub{Runner: runner, Dir: dir}
}

func (github *GitHub) command(arguments ...string) process.Command {
	return process.Command{Name: PROGRAM, Arguments: arguments, Dir: github.Dir}
}

func (github *GitHub) output(context context.Context, arguments ...string) (string, error) {
	return github.Runner.Output(context, github.command(arguments...))
}

// IsLoggedIn reports whether gh is logged in.
func (github *GitHub) IsLoggedIn(context context.Context) bool {
	_, statusError := github.output(context, "auth", "status")

	return statusError == nil
}

// HasRelease reports whether a release exists for a tag.
func (github *GitHub) HasRelease(context context.Context, tag string) bool {
	_, viewError := github.output(context, "release", "view", tag, "--json", "tagName")

	return viewError == nil
}

// ReleaseNotes returns the notes of an existing release.
func (github *GitHub) ReleaseNotes(context context.Context, tag string) (string, error) {
	return github.output(context, "release", "view", tag, "--json", "body", "--jq", ".body")
}

// DeleteRelease deletes a release but keeps its tag.
func (github *GitHub) DeleteRelease(context context.Context, tag string) error {
	_, deleteError := github.output(context, "release", "delete", tag, "--yes")

	return deleteError
}

// Release describes a release to create.
type Release struct {
	Tag   string
	Title string
	// Notes is the body. Empty means notes are generated from the commits.
	Notes string
	Files []string
}

// CreateRelease creates a release for an existing tag. Its output goes to
// output, or to the runner's streams when output is nil.
func (github *GitHub) CreateRelease(context context.Context, output io.Writer, release Release) error {
	arguments := []string{"release", "create", release.Tag, "--verify-tag", "--title", release.Title}

	if release.Notes == "" {
		arguments = append(arguments, "--generate-notes")
	} else {
		arguments = append(arguments, "--notes", release.Notes)
	}

	arguments = append(arguments, release.Files...)

	command := github.command(arguments...)
	command.Output = output

	return github.Runner.Run(context, command)
}

var runPattern = regexp.MustCompile(`https?://\S+/actions/runs/(\d+)`)

// Run is a started workflow run.
type Run struct {
	ID  string
	URL string
}

// StartWorkflow dispatches a workflow on a ref, with inputs, and returns its run.
func (github *GitHub) StartWorkflow(context context.Context, workflow string, ref string, inputs []string) (Run, error) {
	arguments := []string{"workflow", "run", workflow, "--ref", ref}

	for _, input := range inputs {
		arguments = append(arguments, "--raw-field", input)
	}

	output, runError := github.output(context, arguments...)
	if runError != nil {
		return Run{}, runError
	}

	match := runPattern.FindStringSubmatch(output)
	if match == nil {
		return Run{}, errors.New("gh did not return the run URL; update gh")
	}

	return Run{ID: match[1], URL: match[0]}, nil
}

// WatchRun follows a run until it ends, and fails if the run fails. Its
// output goes to output, or to the runner's streams when output is nil.
func (github *GitHub) WatchRun(context context.Context, output io.Writer, id string) error {
	command := github.command("run", "watch", id, "--exit-status")
	command.Output = output

	return github.Runner.Run(context, command)
}

// RunState is the latest run of a workflow for a commit.
type RunState struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
}

// LatestRun returns the latest run of a workflow for a commit, or false.
func (github *GitHub) LatestRun(context context.Context, workflow string, commit string) (RunState, bool, error) {
	output, listError := github.output(context,
		"run", "list", "--workflow", workflow, "--commit", commit,
		"--limit", "1", "--json", "status,conclusion,url")
	if listError != nil {
		return RunState{}, false, listError
	}

	var runs []RunState

	decodeError := json.Unmarshal([]byte(output), &runs)
	if decodeError != nil {
		return RunState{}, false, fmt.Errorf("gh run list: %w", decodeError)
	}

	if len(runs) == 0 {
		return RunState{}, false, nil
	}

	return runs[0], true, nil
}
