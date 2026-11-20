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
// branch, brings in what was tested, the suffix branch's last pre-release,
// and releases it there. Then it brings the release into the suffix branch
// and switches back.
//
// When the stable branch has commits the suffix branch lacks, it asks: merge
// them in, or replace the stable branch with the suffix branch, after
// backing it up. A local
// failure puts the stable branch back and switches back; a publish failure
// stays on the stable branch, as nothing is undone once publishing starts.
func (app *App) flowStable(context context.Context, target *release.Target) error {
	// Read here: with git tags, the stable branch can't see this pre-release.
	pre, readError := target.ReadCurrent(context)
	if readError != nil {
		return readError
	}

	run, runError := newStableRun(context, target, app.UI, pre.Version)
	if runError != nil {
		return runError
	}

	joinError := app.chooseJoin(context, run)
	if joinError != nil {
		return joinError
	}

	backupError := run.backup(context)
	if backupError != nil {
		return backupError
	}

	moveError := run.moveToStable(context)
	if moveError != nil {
		return run.undo(context, moveError)
	}

	plan, result, bumpError := app.bumpStable(context, target, run.from, run.replaces())
	if bumpError != nil {
		return run.undo(context, bumpError)
	}

	prepareError := app.prepare(context, plan, result)
	if prepareError != nil {
		return run.undo(context, prepareError)
	}

	publishError := app.publish(context, plan)
	if publishError != nil {
		return fmt.Errorf("%w\n%s", publishError, run.publishHint(plan))
	}

	return run.update(context)
}

// bumpStable opens the target again on the stable branch, so push and the
// version files are read there, then checks and bumps final from the
// suffix branch's pre-release.
func (app *App) bumpStable(context context.Context, target *release.Target, from semver.Version, replaces string) (*release.Release, *bump.Result, error) {
	stableTarget, openError := release.Open(context, app.Runner, target.Config.Dir)
	if openError != nil {
		return nil, nil, openError
	}

	plan, planError := release.NewFinal(context, stableTarget, from)
	if planError != nil {
		return nil, nil, planError
	}

	plan.Replaces = replaces

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
