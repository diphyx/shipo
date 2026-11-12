// Package publish sends a release out: push, release, workflow and command.
// Publish never reverts.
package publish

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/text/keyword"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/tool/github"
	"github.com/diphyx/shipo/internal/tool/process"
	"github.com/diphyx/shipo/internal/ui"
)

// The publish tasks, in run order.
const (
	TASK_PUSH     = "push"
	TASK_RELEASE  = "release"
	TASK_WORKFLOW = "workflow"
	TASK_COMMAND  = "command"
)

// Tasks lists the tasks in run order.
var Tasks = []string{TASK_PUSH, TASK_RELEASE, TASK_WORKFLOW, TASK_COMMAND}

// ErrNeedsPush is returned by release and workflow when push is off.
var ErrNeedsPush = errors.New("needs push; publish.push is false")

// IsSet reports whether a task is configured.
func IsSet(target *release.Target, task string) bool {
	settings := target.Config.Publish

	switch task {
	case TASK_PUSH:
		{
			return settings.Push.Enabled()
		}
	case TASK_RELEASE:
		{
			return settings.Release.Enabled() && settings.Push.Enabled()
		}
	case TASK_WORKFLOW:
		{
			return settings.Workflow.Enabled() && settings.Push.Enabled()
		}
	case TASK_COMMAND:
		{
			return len(settings.Command) > 0
		}
	}

	return false
}

// Programs lists the programs a task needs on the PATH.
func Programs(target *release.Target, task string) []string {
	switch task {
	case TASK_PUSH:
		{
			return []string{"git"}
		}
	case TASK_RELEASE, TASK_WORKFLOW:
		{
			return []string{"gh"}
		}
	case TASK_COMMAND:
		{
			var names []string

			for _, line := range target.Config.Publish.Command {
				command, parseError := process.Parse(line)
				if parseError == nil && !strings.ContainsAny(command.Name, `/\`) {
					names = append(names, command.Name)
				}
			}

			return names
		}
	}

	return nil
}

// Describe returns what a task will do, with the real values, one line per entry.
func Describe(plan *release.Release, task string) []string {
	values := plan.Values()
	settings := plan.Config.Publish

	switch task {
	case TASK_PUSH:
		{
			var lines []string

			for _, target := range settings.Push.Items {
				lines = append(lines, target+" + "+plan.Tag)
			}

			return lines
		}
	case TASK_RELEASE:
		{
			if settings.Release.IsOn {
				return []string{plan.Tag}
			}

			return []string{plan.Tag + " + " + strings.Join(settings.Release.Items, ", ")}
		}
	case TASK_WORKFLOW:
		{
			return keyword.ExpandAll(settings.Workflow.Items, values)
		}
	case TASK_COMMAND:
		{
			return keyword.ExpandAll(settings.Command, values)
		}
	}

	return nil
}

// Push pushes the release commit and tag to every push entry.
func Push(context context.Context, plan *release.Release, output *ui.Terminal) error {
	current, branchError := plan.Git.Branch(context)
	if branchError != nil {
		return branchError
	}

	for _, entry := range plan.Config.Publish.Push.Items {
		remote, branch, _ := config.SplitPush(entry)

		refspecs := []string{"HEAD:refs/heads/" + branch, "refs/tags/" + plan.Tag}
		if current != "" && current != branch {
			refspecs = append(refspecs, "refs/heads/"+current)
		}

		task := output.Start("push", entry+" + "+plan.Tag)

		pushError := plan.Git.PushAtomic(context, task, remote, refspecs...)
		if pushError != nil {
			task.Fail()

			return fmt.Errorf("push to %s: %w", entry, pushError)
		}

		task.Done()
	}

	_, mainBranch, _ := plan.Push()
	if current != "" && current != mainBranch && plan.Git.Resolve(context, mainBranch) != "" {
		head, _ := plan.Git.Head(context)

		if plan.Git.IsAncestor(context, mainBranch, head) {
			_ = plan.Git.MoveBranch(context, mainBranch, head)
		}
	}

	return nil
}

// ReleaseFiles expands the release paths into files. Each path must match.
func ReleaseFiles(target *release.Target) ([]string, error) {
	var files []string

	for _, path := range target.Config.Publish.Release.Items {
		pattern := filepath.Join(target.Config.Dir, filepath.FromSlash(path))

		info, statError := os.Stat(pattern)
		if statError == nil && info.IsDir() {
			pattern = filepath.Join(pattern, "*")
		}

		matches, globError := filepath.Glob(pattern)
		if globError != nil {
			return nil, fmt.Errorf("publish.release %q: %w", path, globError)
		}

		var regular []string

		for _, match := range matches {
			matchInfo, matchError := os.Stat(match)
			if matchError == nil && !matchInfo.IsDir() {
				regular = append(regular, match)
			}
		}

		if len(regular) == 0 {
			return nil, fmt.Errorf("publish.release %q matches no file", path)
		}

		sort.Strings(regular)
		files = append(files, regular...)
	}

	return files, nil
}

// Release creates the GitHub Release. With current, an existing release is
// re-created and its notes are kept.
func Release(context context.Context, plan *release.Release, output *ui.Terminal) error {
	if !plan.Config.Publish.Push.Enabled() {
		return ErrNeedsPush
	}

	files, filesError := ReleaseFiles(plan.Target)
	if filesError != nil {
		return filesError
	}

	notes := ""

	if plan.GitHub.HasRelease(context, plan.Tag) {
		if plan.Level != semver.LEVEL_CURRENT {
			return fmt.Errorf("release %s already exists", plan.Tag)
		}

		kept, notesError := plan.GitHub.ReleaseNotes(context, plan.Tag)
		if notesError != nil {
			return notesError
		}

		notes = kept

		deleteError := plan.GitHub.DeleteRelease(context, plan.Tag)
		if deleteError != nil {
			return deleteError
		}
	}

	detail := plan.Tag
	if len(files) > 0 {
		detail += fmt.Sprintf(" + %d file(s)", len(files))
	}

	task := output.Start("release", detail)

	createError := plan.GitHub.CreateRelease(context, task, github.Release{Tag: plan.Tag, Title: plan.Tag, Notes: notes, Files: files})
	if createError != nil {
		task.Fail()

		return createError
	}

	task.Done()

	return nil
}

// Workflow starts every workflow in order, and waits for each unless isNoWait.
func Workflow(context context.Context, plan *release.Release, output *ui.Terminal, isNoWait bool) error {
	_, branch, isPushed := plan.Push()
	if !isPushed {
		return ErrNeedsPush
	}

	for _, entry := range keyword.ExpandAll(plan.Config.Publish.Workflow.Items, plan.Values()) {
		file, inputs, parseError := ParseWorkflow(entry)
		if parseError != nil {
			return parseError
		}

		run, startError := plan.GitHub.StartWorkflow(context, file, branch, inputs)
		if startError != nil {
			return fmt.Errorf("workflow %s: %w", file, startError)
		}

		if isNoWait {
			output.Done("workflow", file+" started "+run.URL)

			continue
		}

		task := output.Start("workflow", file+" "+run.URL)

		watchError := plan.GitHub.WatchRun(context, task, run.ID)
		if watchError != nil {
			task.Fail()

			return fmt.Errorf("workflow %s did not succeed: %s", file, run.URL)
		}

		task.DoneAs(file)
	}

	return nil
}

// ParseWorkflow splits a workflow entry into its file and name=value inputs.
func ParseWorkflow(entry string) (string, []string, error) {
	arguments, splitError := process.Split(entry)
	if splitError != nil {
		return "", nil, fmt.Errorf("publish.workflow %q: %w", entry, splitError)
	}

	for _, input := range arguments[1:] {
		equals := strings.IndexByte(input, '=')
		if equals <= 0 {
			return "", nil, fmt.Errorf("publish.workflow %q: input %q is not name=value", entry, input)
		}
	}

	return arguments[0], arguments[1:], nil
}

// Command runs every publish command in order, in the .shipo folder.
func Command(context context.Context, plan *release.Release, output *ui.Terminal) error {
	lines := keyword.ExpandAll(plan.Config.Publish.Command, plan.Values())

	for index, line := range lines {
		command, parseError := process.Parse(line)
		if parseError != nil {
			return fmt.Errorf("publish.command %q: %w", line, parseError)
		}

		command.Dir = plan.Config.Dir
		task := output.Start("command", fmt.Sprintf("[%d/%d] %s", index+1, len(lines), line))
		command.Output = task

		runError := plan.Runner.Run(context, command)
		if runError != nil {
			task.Fail()

			return runError
		}

		task.Done()
	}

	return nil
}
