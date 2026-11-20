package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/ui"
)

// The ways the suffix branch goes to the stable branch, when the stable
// branch has commits it lacks.
const (
	JOIN_MERGE   = "merge"
	JOIN_REPLACE = "replace"
)

// chooseJoin asks how the source goes to the stable branch when the stable
// branch has commits it lacks: merge them in, or replace the stable branch
// with exactly the source, as tested. With none, the merge is a
// fast-forward and there is nothing to ask.
func (app *App) chooseJoin(context context.Context, run *stableRun) error {
	if run.lacking == 0 {
		return nil
	}

	if !app.UI.IsInteractive {
		return ui.ErrNoTerminal
	}

	dropped := release.Commits(run.lacking)
	if run.lacking == 1 {
		dropped = strings.Fields(run.missing[0])[0]
	}

	title := fmt.Sprintf("%s has %s %s lacks", run.stable, release.Commits(run.lacking), run.sourceName)
	merge := run.stable + " + " + run.sourceName + ", with a merge commit"
	if !run.canMerge {
		merge = "conflicts; merge " + run.stable + " into " + run.branch + " by hand first"
	}

	choices := []ui.Choice{
		{Label: JOIN_MERGE, Detail: merge},
		{Label: JOIN_REPLACE, Detail: run.stable + " becomes " + run.sourceName + "; drops " + dropped + ", backed up"},
	}

	index, menuError := app.UI.Menu(context, title, choices)
	if menuError != nil {
		return menuError
	}

	run.isReplaced = choices[index].Label == JOIN_REPLACE

	if !run.isReplaced && !run.canMerge {
		return fmt.Errorf("%s and %s conflict; merge %s into %s and resolve it, then run final again", run.sourceName, run.stable, run.stable, run.branch)
	}

	return nil
}

// backup saves the stable branch before replace drops its commits, as
// backup/<stable>-<date>, pushed with the release's push, so nothing it drops
// is lost. When the local and the remote one diverged, both are saved, the
// local one with -local. It changes nothing else, and stays after an undo.
func (run *stableRun) backup(context context.Context) error {
	if !run.isReplaced {
		return nil
	}

	remote, _, isPushed := run.target.Push()

	// Each entry is a commit to save and its name's suffix.
	var refs [][2]string

	if run.stableRemote != "" {
		refs = append(refs, [2]string{run.stableRemote, ""})
	}

	isLocalOnly := run.stableBefore != "" && (run.stableRemote == "" || !run.git.IsAncestor(context, run.stableBefore, run.stableRemote))
	if isLocalOnly {
		suffix := ""
		if run.stableRemote != "" {
			suffix = "-local"
		}

		refs = append(refs, [2]string{run.stableBefore, suffix})
	}

	for _, entry := range refs {
		name := run.backupName(context, remote, entry[1])

		createError := run.git.CreateBranch(context, name, entry[0])
		if createError != nil {
			return fmt.Errorf("back up %s as %s: %w", run.stable, name, createError)
		}

		run.backups = append(run.backups, name)

		if !isPushed {
			run.output.Done("backup", name)

			continue
		}

		task := run.output.Start("backup", name+", push to "+remote)

		pushError := run.git.PushAtomic(context, task, remote, "refs/heads/"+name)
		if pushError != nil {
			task.Fail()

			return fmt.Errorf("push %s: %w; nothing else changed", name, pushError)
		}

		task.Done()
	}

	return nil
}

// backupName is backup/<stable>-<date><suffix>, with -2, -3… when taken,
// locally or on the remote, so an earlier backup is never overwritten.
func (run *stableRun) backupName(context context.Context, remote string, suffix string) string {
	base := "backup/" + run.stable + "-" + time.Now().Format("2006-01-02") + suffix
	name := base

	for count := 2; run.git.Resolve(context, name) != "" || run.git.Resolve(context, remote+"/"+name) != ""; count++ {
		name = fmt.Sprintf("%s-%d", base, count)
	}

	return name
}

// moveToStable switches to the stable branch and brings the source in: a
// merge, a fast-forward when nothing is lacking, or for replace, the stable
// branch reset to the source, after listing what it drops.
func (run *stableRun) moveToStable(context context.Context) error {
	switchError := run.git.Switch(context, run.stable)
	if switchError != nil {
		return fmt.Errorf("switch to %s: %w", run.stable, switchError)
	}

	run.output.Done("switch", run.stable)

	if run.isReplaced {
		resetError := run.git.ResetHard(context, run.source)
		if resetError != nil {
			return fmt.Errorf("replace %s with %s: %w", run.stable, run.sourceName, resetError)
		}

		run.output.Skip("drop", fmt.Sprintf("%s of %s, kept in %s", release.Commits(run.lacking), run.stable, strings.Join(run.backups, " and ")))

		run.output.Done("replace", run.stable+" = "+run.sourceName)

		return nil
	}

	// A local stable branch behind its remote catches up first, so the merge
	// brings in what the menu counted, and the bump finds nothing to merge.
	if run.stableRef != run.stable {
		forwardError := run.git.FastForward(context, run.stableRef)
		if forwardError != nil {
			return fmt.Errorf("update %s to %s: %w", run.stable, run.stableRef, forwardError)
		}
	}

	mergeError := run.git.MergeAs(context, run.source, "Merge "+run.sourceName+" into "+run.stable)
	if mergeError != nil {
		return fmt.Errorf("merge %s: %w", run.sourceName, mergeError)
	}

	detail := run.sourceName + " (" + release.Commits(run.ahead) + ")"
	if run.lacking > 0 {
		detail += fmt.Sprintf(" + %s's %d", run.stable, run.lacking)
	}

	run.output.Done("merge", detail)

	return nil
}
