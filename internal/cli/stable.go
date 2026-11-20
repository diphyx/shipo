package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/diphyx/shipo/internal/check"
	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/scope/bump"
	"github.com/diphyx/shipo/internal/text/semver"
)

// flowStable runs final from a suffix branch: it switches to the stable
// branch, merges the suffix branch in, and releases there. Then it brings
// the suffix branch up to the release and switches back. A local failure
// resets the stable branch and switches back; a publish failure stays on
// the stable branch, as nothing is undone once publishing starts.
func (app *App) flowStable(context context.Context, target *release.Target) error {
	branch := target.HeadBranch
	stable := target.Config.Bump.Stable
	repository := target.Git

	readyError := stableReady(context, target)
	if readyError != nil {
		return readyError
	}

	// Read here: with git tags, the stable branch can't see this pre-release.
	pre, readError := target.ReadCurrent(context)
	if readError != nil {
		return readError
	}

	final, _ := pre.Version.Bump(semver.LEVEL_FINAL)

	tag := target.Version.TagName(target.Folder, final.String())
	if repository.HasTag(context, tag) {
		return fmt.Errorf("final: %s already exists; bump %s further first", tag, branch)
	}

	before := repository.Resolve(context, stable)
	commits, _ := repository.CountCommits(context, stable, branch)

	switchError := repository.Switch(context, stable)
	if switchError != nil {
		return fmt.Errorf("switch to %s: %w", stable, switchError)
	}

	app.UI.Done("switch", stable)

	undo := func(cause error) error {
		detached := withoutCancel(context)

		var undoErrors []error

		if before != "" {
			undoErrors = append(undoErrors, repository.ResetHard(detached, before))
		}

		undoErrors = append(undoErrors, repository.Switch(detached, branch))
		app.UI.Fail("switch", branch+"; "+stable+" undone")

		return errors.Join(append([]error{cause}, undoErrors...)...)
	}

	mergeError := repository.Merge(context, branch)
	if mergeError != nil {
		return undo(fmt.Errorf("merge %s: %w", branch, mergeError))
	}

	app.UI.Done("merge", branch+" ("+release.Commits(commits)+")")

	plan, result, bumpError := app.bumpStable(context, target, pre.Version)
	if bumpError != nil {
		return undo(bumpError)
	}

	prepareError := app.prepare(context, plan, result)
	if prepareError != nil {
		return undo(prepareError)
	}

	publishError := app.publish(context, plan)
	if publishError != nil {
		return fmt.Errorf("%w\nyou are on %s: fix it, run the publish steps there, then merge %s into %s", publishError, stable, stable, branch)
	}

	return app.updateBranch(context, target, branch, stable)
}

// stableReady checks the suffix branch can go to the stable branch: a clean
// tree, and a merge with no conflict.
func stableReady(context context.Context, target *release.Target) error {
	stable := target.Config.Bump.Stable

	isClean, cleanError := target.Git.IsClean(context)
	if cleanError != nil {
		return cleanError
	}

	if !isClean {
		return fmt.Errorf("the tree has changes; commit or stash them before final switches to %s", stable)
	}

	if target.Git.Resolve(context, stable) != "" && !target.Git.CanMerge(context, stable) {
		return fmt.Errorf("%s and %s conflict; merge %s into %s and resolve it first", target.HeadBranch, stable, stable, target.HeadBranch)
	}

	return nil
}

// bumpStable opens the target again on the stable branch, so push and the
// version files are read there, then checks and bumps final from the
// suffix branch's pre-release.
func (app *App) bumpStable(context context.Context, target *release.Target, from semver.Version) (*release.Release, *bump.Result, error) {
	stableTarget, openError := release.Open(context, app.Runner, target.Config.Dir)
	if openError != nil {
		return nil, nil, openError
	}

	plan, planError := release.NewFinal(context, stableTarget, from)
	if planError != nil {
		return nil, nil, planError
	}

	app.UI.Done("bump", bumpDetail(plan))

	checksError := app.runChecks(context, plan, check.Everything())
	if checksError != nil {
		return nil, nil, checksError
	}

	result, bumpError := bump.Create(context, plan, app.UI)
	if bumpError != nil {
		return nil, nil, bumpError
	}

	return plan, result, nil
}

// updateBranch switches back to the suffix branch, fast-forwards it to the
// release on the stable branch, and pushes it where it pushes.
func (app *App) updateBranch(context context.Context, target *release.Target, from string, stable string) error {
	repository := target.Git

	switchError := repository.Switch(context, from)
	if switchError != nil {
		return fmt.Errorf("switch to %s: %w", from, switchError)
	}

	forwardError := repository.FastForward(context, stable)
	if forwardError != nil {
		return fmt.Errorf("update %s to %s: %w", from, stable, forwardError)
	}

	remote, _, isPushed := target.Push()
	if !isPushed {
		app.UI.Done("update", from+" ← "+stable)
		app.UI.Done("switch", from)

		return nil
	}

	task := app.UI.Start("update", from+" ← "+stable+", push to "+remote)

	pushError := repository.PushAtomic(context, task, remote, "refs/heads/"+from)
	if pushError != nil {
		task.Fail()

		return fmt.Errorf("push %s: %w", from, pushError)
	}

	task.Done()
	app.UI.Done("switch", from)

	return nil
}

// withoutCancel keeps the values of parent but not its cancel, so an undo
// still runs after Ctrl-C.
func withoutCancel(parent context.Context) context.Context {
	return context.WithoutCancel(parent)
}
