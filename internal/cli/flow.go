package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/diphyx/shipo/internal/check"
	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/scope/build"
	"github.com/diphyx/shipo/internal/scope/bump"
	"github.com/diphyx/shipo/internal/scope/publish"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/ui"
)

// Flow runs the whole flow, interactively. An empty level uses [bump] level.
func (app *App) Flow(context context.Context, level semver.Level) error {
	if !app.UI.IsInteractive {
		return ui.ErrNoTerminal
	}

	target, isPicked, openError := app.openTarget(context, true)
	if openError != nil {
		return openError
	}

	if isPicked {
		app.UI.Done("target", target.Folder)
	}

	retry, retryError := app.offerRetry(context, target)
	if retryError != nil {
		return retryError
	}

	if retry != nil {
		return app.finish(context, retry, nil)
	}

	chosen, levelError := app.chooseLevel(context, target, level)
	if levelError != nil {
		return levelError
	}

	plan, planError := release.NewRelease(context, target, chosen)
	if planError != nil {
		return planError
	}

	app.UI.Done("bump", bumpDetail(plan))

	checksError := app.runChecks(context, plan, check.Everything())
	if checksError != nil {
		return checksError
	}

	result, bumpError := bump.Create(context, plan, app.UI)
	if bumpError != nil {
		return bumpError
	}

	return app.finish(context, plan, result)
}

// finish runs build, then publish. A build failure reverts the bump.
func (app *App) finish(context context.Context, plan *release.Release, result *bump.Result) error {
	if build.IsSet(plan.Target) {
		buildError := build.Run(context, plan, app.UI)
		if buildError != nil {
			if result != nil {
				revertError := bump.Revert(context, plan.Target, result)
				app.UI.Fail("build", "failed; bump undone")

				return errors.Join(buildError, revertError)
			}

			return buildError
		}
	}

	if publish.IsSet(plan.Target, publish.TASK_RELEASE) {
		_, filesError := publish.ReleaseFiles(plan.Target)
		if filesError != nil {
			if result != nil {
				revertError := bump.Revert(context, plan.Target, result)

				return errors.Join(filesError, revertError)
			}

			return filesError
		}
	}

	return app.publish(context, plan)
}

func (app *App) publish(context context.Context, plan *release.Release) error {
	for _, task := range publish.Tasks {
		if !publish.IsSet(plan.Target, task) {
			continue
		}

		if task != publish.TASK_PUSH {
			app.UI.Print("\n")
			app.UI.Block(task, publish.Describe(plan, task))

			isYes, confirmError := app.UI.Confirm(context, question(task))
			if confirmError != nil {
				return confirmError
			}

			if !isYes {
				app.UI.Skip(task, "skipped")

				continue
			}
		}

		runError := runTask(context, plan, app.UI, task, false)
		if runError != nil {
			app.UI.Fail(task, "failed; nothing was undone")

			return runError
		}
	}

	return nil
}

func question(task string) string {
	if task == publish.TASK_RELEASE {
		return "Create?"
	}

	return "Run?"
}

func runTask(context context.Context, plan *release.Release, output *ui.Terminal, task string, isNoWait bool) error {
	switch task {
	case publish.TASK_PUSH:
		{
			return publish.Push(context, plan, output)
		}
	case publish.TASK_RELEASE:
		{
			return publish.Release(context, plan, output)
		}
	case publish.TASK_WORKFLOW:
		{
			return publish.Workflow(context, plan, output, isNoWait)
		}
	case publish.TASK_COMMAND:
		{
			return publish.Command(context, plan, output)
		}
	}

	return fmt.Errorf("unknown task %q", task)
}

// chooseLevel returns the level from the argument, [bump] level, or the menu.
func (app *App) chooseLevel(context context.Context, target *release.Target, level semver.Level) (semver.Level, error) {
	if level == "" {
		parsed, parseError := semver.ParseLevel(target.Config.Bump.Level)
		if parseError != nil {
			return "", fmt.Errorf("bump.level: %w", parseError)
		}

		level = parsed
	}

	if level != semver.LEVEL_ASK {
		return level, nil
	}

	current, readError := target.ReadCurrent(context)
	if readError != nil {
		return "", readError
	}

	choices := make([]ui.Choice, len(semver.Levels))

	for index, option := range semver.Levels {
		next, _ := current.Version.Bump(option)
		choices[index] = ui.Choice{Label: string(option), Detail: next.String()}
	}

	index, menuError := app.UI.Menu(context, fmt.Sprintf("%s (%s)", target.Version.Preset, current.Version), choices)
	if menuError != nil {
		return "", menuError
	}

	return semver.Levels[index], nil
}

// offerRetry offers to publish HEAD again when it is a release whose tag
// hasn't reached the remote.
func (app *App) offerRetry(context context.Context, target *release.Target) (*release.Release, error) {
	remote, _, isPushed := target.Push()
	if !isPushed {
		return nil, nil
	}

	plan, headError := release.FromHead(context, target)
	if headError != nil {
		return nil, nil
	}

	isRemote, _ := target.Git.HasRemoteTag(context, remote, plan.Tag)
	if isRemote {
		return nil, nil
	}

	isYes, confirmError := app.UI.Confirm(context, fmt.Sprintf("HEAD is release %s, not yet pushed. Publish it again without a new bump?", plan.Tag))
	if confirmError != nil || !isYes {
		return nil, confirmError
	}

	checksError := app.runChecks(context, plan, check.Options{IsBuild: true, Tasks: publish.Tasks})
	if checksError != nil {
		return nil, checksError
	}

	return plan, nil
}

// runChecks runs the checks on a live line that shows each one as it starts,
// then prints what passed, or each failure on its own line.
func (app *App) runChecks(context context.Context, plan *release.Release, options check.Options) error {
	task := app.UI.Start("checks", "")

	if app.UI.IsLive {
		options.Progress = task
	}

	items := check.Run(context, plan, options)
	failed := check.Failed(items)

	if len(failed) == 0 {
		task.DoneAs(checkNames(items))

		return nil
	}

	task.FailAs(checkNames(failed))

	for _, item := range failed {
		app.UI.Item(ui.MARK_FAIL, item.Name, item.Detail)
	}

	return fmt.Errorf("checks failed: %s", checkNames(failed))
}

// checkNames lists the names of checks, each once, in order.
func checkNames(items []check.Item) string {
	var names []string

	seen := map[string]bool{}

	for _, item := range items {
		if !seen[item.Name] {
			seen[item.Name] = true
			names = append(names, item.Name)
		}
	}

	return strings.Join(names, ", ")
}

// bumpDetail describes the chosen bump: "minor · 1.1.0 → 1.2.0".
func bumpDetail(plan *release.Release) string {
	if plan.Level == semver.LEVEL_CURRENT {
		return fmt.Sprintf("%s · %s", plan.Level, plan.Next)
	}

	return fmt.Sprintf("%s · %s → %s", plan.Level, plan.Previous, plan.Next)
}
