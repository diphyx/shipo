package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/diphyx/shipo/internal/check"
	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/scope/build"
	"github.com/diphyx/shipo/internal/scope/bump"
	"github.com/diphyx/shipo/internal/scope/publish"
	"github.com/diphyx/shipo/internal/text/semver"
)

// StepBump runs only the bump, with no prompt.
func (app *App) StepBump(context context.Context, name string) error {
	level, parseError := semver.ParseLevel(name)
	if parseError != nil {
		return parseError
	}

	if level == semver.LEVEL_ASK {
		return errors.New("shipo bump needs a level: major, minor, patch or current")
	}

	target, openError := release.Open(context, app.Runner, app.Dir)
	if openError != nil {
		return openError
	}

	plan, planError := release.NewRelease(context, target, level)
	if planError != nil {
		return planError
	}

	checksError := app.runChecks(context, plan, check.Options{IsBump: true, Tasks: []string{publish.TASK_PUSH}})
	if checksError != nil {
		return checksError
	}

	_, bumpError := bump.Create(context, plan, app.UI)

	return bumpError
}

// Step runs one step on the release that HEAD already is, with no prompt.
func (app *App) Step(context context.Context, name string, isNoWait bool) error {
	target, openError := release.Open(context, app.Runner, app.Dir)
	if openError != nil {
		return openError
	}

	plan, headError := release.FromHead(context, target)
	if headError != nil {
		return headError
	}

	if name == "build" {
		if !build.IsSet(target) {
			app.UI.Skip("build", "not set")

			return nil
		}

		checkError := build.Check(target)
		if checkError != nil {
			return checkError
		}

		return build.Run(context, plan, app.UI)
	}

	if !publish.IsSet(target, name) {
		if name != publish.TASK_PUSH && !target.Config.Publish.Push.Enabled() {
			return fmt.Errorf("%s: %w", name, publish.ErrNeedsPush)
		}

		app.UI.Skip(name, "not set")

		return nil
	}

	if name == publish.TASK_RELEASE || name == publish.TASK_WORKFLOW {
		remote, _, _ := target.Push()

		isRemote, _ := target.Git.HasRemoteTag(context, remote, plan.Tag)
		if !isRemote {
			return fmt.Errorf("%s: tag %s is not on %s; run shipo push first", name, plan.Tag, remote)
		}
	}

	options := check.Options{Tasks: []string{name}}

	checksError := app.runChecks(context, plan, options)
	if checksError != nil {
		return checksError
	}

	return runTask(context, plan, app.UI, name, isNoWait)
}
