package cli

import (
	"context"
	"fmt"

	"github.com/diphyx/shipo/internal/check"
	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/scope/bump"
	"github.com/diphyx/shipo/internal/text/semver"
)

// flowStable runs final from a suffix branch: it switches to the stable
// branch, brings the suffix branch in, and releases there. Then it brings
// the suffix branch up to the release and switches back.
//
// When the stable branch has commits the suffix branch lacks, it asks how
// to join them: merge, or rebase the suffix branch on top. A local failure
// puts both branches back and switches back; a publish failure stays on the
// stable branch, as nothing is undone once publishing starts.
func (app *App) flowStable(context context.Context, target *release.Target) error {
	// Read here: with git tags, the stable branch can't see this pre-release.
	pre, readError := target.ReadCurrent(context)
	if readError != nil {
		return readError
	}

	run, runError := newStableRun(context, target, app.UI)
	if runError != nil {
		return runError
	}

	join, joinError := app.chooseJoin(context, run)
	if joinError != nil {
		return joinError
	}

	if join == JOIN_REBASE {
		rebaseError := run.rebase(context)
		if rebaseError != nil {
			return rebaseError
		}
	}

	moveError := run.moveToStable(context)
	if moveError != nil {
		return run.undo(context, moveError)
	}

	plan, result, bumpError := app.bumpStable(context, target, pre.Version)
	if bumpError != nil {
		return run.undo(context, bumpError)
	}

	prepareError := app.prepare(context, plan, result)
	if prepareError != nil {
		return run.undo(context, prepareError)
	}

	publishError := app.publish(context, plan)
	if publishError != nil {
		return fmt.Errorf("%w\n%s", publishError, run.publishHint())
	}

	return run.update(context)
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
