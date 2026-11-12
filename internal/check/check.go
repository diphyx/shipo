// Package check runs the checks before any change, and reads the state of a
// release from git and GitHub. Nothing is stored.
package check

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/scope/build"
	"github.com/diphyx/shipo/internal/scope/publish"
	"github.com/diphyx/shipo/internal/text/semver"
)

// Item is one check or state line.
type Item struct {
	Name   string
	IsOK   bool
	Detail string
}

// Failed returns the items that are not OK.
func Failed(items []Item) []Item {
	var failed []Item

	for _, item := range items {
		if !item.IsOK {
			failed = append(failed, item)
		}
	}

	return failed
}

// Error joins failed items into one error, or returns nil.
func Error(items []Item) error {
	failed := Failed(items)
	if len(failed) == 0 {
		return nil
	}

	lines := make([]string, len(failed))
	for index, item := range failed {
		lines[index] = item.Name + ": " + item.Detail
	}

	return fmt.Errorf("checks failed:\n  %s", strings.Join(lines, "\n  "))
}

// Options says which parts of the flow the checks cover.
type Options struct {
	// IsBump checks what bump needs: a clean tree and a free tag.
	IsBump bool
	// IsBuild checks the build.
	IsBuild bool
	// Tasks are the publish tasks to check.
	Tasks []string
	// Progress, when set, gets one line for each check as it starts.
	Progress io.Writer
}

func (options Options) report(text string) {
	if options.Progress != nil {
		_, _ = fmt.Fprintln(options.Progress, text)
	}
}

// Everything checks the whole flow.
func Everything() Options {
	return Options{IsBump: true, IsBuild: true, Tasks: publish.Tasks}
}

// Run runs the checks for a planned release. It fetches the push remotes.
func Run(context context.Context, plan *release.Release, options Options) []Item {
	target := plan.Target

	var items []Item

	options.report("checking tools")
	items = append(items, tools(context, target, options))

	if options.IsBump {
		options.report("checking the working tree")

		isClean, cleanError := target.Git.IsClean(context)
		items = append(items, Item{Name: "tree", IsOK: cleanError == nil && isClean, Detail: pick(isClean, "clean", "has changes; commit or stash them")})

		_, locksError := target.Version.Locks()
		if locksError != nil {
			items = append(items, Item{Name: "lock", Detail: locksError.Error()})
		}
	}

	if publish.IsSet(target, publish.TASK_PUSH) && containsTask(options.Tasks, publish.TASK_PUSH) {
		items = append(items, remotes(context, plan, options)...)
	}

	if options.IsBuild && build.IsSet(target) {
		options.report("checking the build")

		buildError := build.Check(target)
		items = append(items, Item{Name: "build", IsOK: buildError == nil, Detail: errorOr(buildError, target.Config.Rel(build.Dir(target)))})
	}

	if publish.IsSet(target, publish.TASK_WORKFLOW) && containsTask(options.Tasks, publish.TASK_WORKFLOW) {
		options.report("checking the workflows")
		items = append(items, workflows(target)...)
	}

	return items
}

// workflows checks each workflow entry parses, and that a workflow file in
// .github/workflows can be dispatched.
func workflows(target *release.Target) []Item {
	var items []Item

	for _, entry := range target.Config.Publish.Workflow.Items {
		file, _, parseError := publish.ParseWorkflow(entry)
		if parseError != nil {
			items = append(items, Item{Name: "workflow", Detail: parseError.Error()})

			continue
		}

		extension := filepath.Ext(file)
		if extension != ".yml" && extension != ".yaml" {
			continue
		}

		data, readError := os.ReadFile(filepath.Join(target.Root, ".github", "workflows", file))
		if readError != nil {
			items = append(items, Item{Name: "workflow", Detail: file + " not found in .github/workflows"})

			continue
		}

		if !strings.Contains(string(data), "workflow_dispatch") {
			items = append(items, Item{Name: "workflow", Detail: file + " has no on: workflow_dispatch"})
		}
	}

	return items
}

func tools(context context.Context, target *release.Target, options Options) Item {
	needed := map[string]bool{"git": true}

	if options.IsBump {
		locks, _ := target.Version.Locks()
		for _, lock := range locks {
			needed[lock.Program] = true
		}
	}

	for _, task := range options.Tasks {
		if !publish.IsSet(target, task) {
			continue
		}

		for _, name := range publish.Programs(target, task) {
			needed[name] = true
		}
	}

	var names, missing []string

	for name := range needed {
		names = append(names, name)

		if !target.Runner.Look(name) {
			missing = append(missing, name)
		}
	}

	sort.Strings(names)
	sort.Strings(missing)

	if len(missing) > 0 {
		return Item{Name: "tools", Detail: "not installed: " + strings.Join(missing, ", ")}
	}

	if needed["gh"] && !target.GitHub.IsLoggedIn(context) {
		return Item{Name: "tools", Detail: "gh is not logged in; run gh auth login"}
	}

	return Item{Name: "tools", IsOK: true, Detail: strings.Join(names, ", ")}
}

func remotes(context context.Context, plan *release.Release, options Options) []Item {
	isBump := options.IsBump
	target := plan.Target

	var items []Item

	head, _ := target.Git.Head(context)

	for _, entry := range target.Config.Publish.Push.Items {
		remote, branch, _ := config.SplitPush(entry)

		if !target.Git.HasRemote(context, remote) {
			items = append(items, Item{Name: "remote", Detail: remote + " is not a configured remote"})

			continue
		}

		options.report("fetching " + remote)

		fetchError := target.Git.Fetch(context, remote)
		if fetchError != nil {
			items = append(items, Item{Name: "remote", Detail: "git fetch " + remote + " failed"})

			continue
		}

		remoteBranch := target.Git.Resolve(context, remote+"/"+branch)

		switch {
		case remoteBranch == "":
			{
				items = append(items, Item{Name: "remote", IsOK: true, Detail: entry + ", new branch"})
			}
		case target.Git.IsAncestor(context, remoteBranch, head):
			{
				items = append(items, Item{Name: "remote", IsOK: true, Detail: entry + ", fast-forward ok"})
			}
		default:
			{
				items = append(items, Item{Name: "remote", Detail: entry + " has commits you don't have; pull first"})
			}
		}

		if isBump && plan.Level != semver.LEVEL_CURRENT {
			options.report("checking tag " + plan.Tag + " on " + remote)

			isTaken, _ := target.Git.HasRemoteTag(context, remote, plan.Tag)
			if isTaken {
				items = append(items, Item{Name: "tag", Detail: plan.Tag + " already exists on " + remote})
			}
		}
	}

	if isBump && plan.Level != semver.LEVEL_CURRENT && target.Git.HasTag(context, plan.Tag) {
		items = append(items, Item{Name: "tag", Detail: plan.Tag + " already exists"})
	}

	return items
}

// State reads where a release stands: its tag, its GitHub Release and its workflows.
func State(context context.Context, plan *release.Release) []Item {
	target := plan.Target

	var items []Item

	isLocal := target.Git.HasTag(context, plan.Tag)
	places := []string{}

	if isLocal {
		places = append(places, "local")
	}

	remote, _, isPushed := plan.Push()
	isRemote := false

	if isPushed && isLocal {
		isRemote, _ = target.Git.HasRemoteTag(context, remote, plan.Tag)
		if isRemote {
			places = append(places, "remote")
		}
	}

	tagDetail := strings.Join(places, ", ")
	if tagDetail == "" {
		tagDetail = "not created"
	} else if isPushed && !isRemote {
		tagDetail += ", not pushed"
	}

	items = append(items, Item{Name: "tag", IsOK: isLocal && (!isPushed || isRemote), Detail: tagDetail})

	if publish.IsSet(target, publish.TASK_RELEASE) && target.Runner.Look("gh") {
		hasRelease := target.GitHub.HasRelease(context, plan.Tag)
		items = append(items, Item{Name: "release", IsOK: hasRelease, Detail: pick(hasRelease, "created", "not created")})
	}

	if publish.IsSet(target, publish.TASK_WORKFLOW) && target.Runner.Look("gh") && isLocal {
		commit := target.Git.Resolve(context, plan.Tag)

		for _, entry := range target.Config.Publish.Workflow.Items {
			file, _, parseError := publish.ParseWorkflow(entry)
			if parseError != nil {
				continue
			}

			run, isFound, _ := target.GitHub.LatestRun(context, file, commit)

			switch {
			case !isFound:
				{
					items = append(items, Item{Name: "workflow", Detail: file + " not run"})
				}
			case run.Status != "completed":
				{
					items = append(items, Item{Name: "workflow", Detail: file + " " + run.Status})
				}
			default:
				{
					items = append(items, Item{Name: "workflow", IsOK: run.Conclusion == "success", Detail: file + " " + run.Conclusion})
				}
			}
		}
	}

	return items
}

func containsTask(tasks []string, task string) bool {
	for _, item := range tasks {
		if item == task {
			return true
		}
	}

	return false
}

func pick(isFirst bool, first string, second string) string {
	if isFirst {
		return first
	}

	return second
}

func errorOr(problem error, fallback string) string {
	if problem != nil {
		return problem.Error()
	}

	return fallback
}
