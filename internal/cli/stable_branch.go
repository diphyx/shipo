package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/tool/git"
	"github.com/diphyx/shipo/internal/ui"
)

// The ways the suffix branch joins the stable branch, when the stable branch
// has commits it lacks.
const (
	JOIN_MERGE  = "merge"
	JOIN_REBASE = "rebase"
)

// stableRun moves between the suffix branch and the stable branch for one
// final, and keeps where both were, to undo.
type stableRun struct {
	target *release.Target
	git    *git.Git
	output *ui.Terminal
	branch string
	stable string
	// branchBefore and stableBefore are the branches' commits at the start;
	// stableBefore is "" when the stable branch isn't local yet.
	branchBefore string
	stableBefore string
	// stableRef is the stable branch to compare and rebase onto: the local
	// one, or the remote's when it is ahead or the local one isn't there yet.
	stableRef string
	// remoteBefore is the suffix branch on its remote: the lease for a
	// force-push after a rebase.
	remoteBefore string
	// ahead counts the suffix branch's commits the stable branch lacks;
	// lacking, the stable branch's commits the suffix branch lacks.
	ahead     int
	lacking   int
	isRebased bool
}

// newStableRun checks the suffix branch can go to the stable branch, a
// clean tree and no conflict, and reads where both stand.
func newStableRun(context context.Context, target *release.Target, output *ui.Terminal) (*stableRun, error) {
	run := &stableRun{
		target: target,
		git:    target.Git,
		output: output,
		branch: target.HeadBranch,
		stable: target.Config.Bump.Stable,
	}

	isClean, cleanError := run.git.IsClean(context)
	if cleanError != nil {
		return nil, cleanError
	}

	if !isClean {
		return nil, fmt.Errorf("the tree has changes; commit or stash them before final switches to %s", run.stable)
	}

	run.branchBefore = run.git.Resolve(context, run.branch)
	run.stableBefore = run.git.Resolve(context, run.stable)
	run.stableRef = run.stable

	remote, _, isPushed := target.Push()
	if isPushed {
		remoteError := run.readRemote(context, remote)
		if remoteError != nil {
			return nil, remoteError
		}
	}

	if run.git.Resolve(context, run.stableRef) == "" {
		return run, nil
	}

	if !run.git.CanMerge(context, run.stableRef) {
		return nil, fmt.Errorf("%s and %s conflict; merge %s into %s and resolve it first", run.branch, run.stable, run.stable, run.branch)
	}

	run.ahead, _ = run.git.CountCommits(context, run.stableRef, run.branch)
	run.lacking, _ = run.git.CountCommits(context, run.branch, run.stableRef)

	return run, nil
}

// readRemote fetches, so commits pushed to either branch since the last
// fetch count. The suffix branch must hold all of its remote's commits: the
// remote's is then the lease for a force-push. The stable branch is compared
// from its remote when the local one is behind it, or not there yet.
func (run *stableRun) readRemote(context context.Context, remote string) error {
	fetchError := run.git.Fetch(context, remote)
	if fetchError != nil {
		return fmt.Errorf("git fetch %s: %w", remote, fetchError)
	}

	remoteBranch := remote + "/" + run.branch
	run.remoteBefore = run.git.Resolve(context, remoteBranch)

	if run.remoteBefore != "" && !run.git.IsAncestor(context, remoteBranch, run.branch) {
		return fmt.Errorf("%s has commits you don't have; pull them into %s first", remoteBranch, run.branch)
	}

	remoteStable := remote + "/" + run.stable
	if run.git.Resolve(context, remoteStable) == "" {
		return nil
	}

	if run.stableBefore == "" || run.git.IsAncestor(context, run.stable, remoteStable) {
		run.stableRef = remoteStable
	}

	return nil
}

// chooseJoin asks how to join the branches when the stable branch has
// commits the suffix branch lacks. With none, the merge is a fast-forward
// and there is nothing to ask.
func (app *App) chooseJoin(context context.Context, run *stableRun) (string, error) {
	if run.lacking == 0 {
		return JOIN_MERGE, nil
	}

	onto := run.stable + "'s " + release.Commits(run.lacking)
	if run.lacking == 1 {
		onto, _ = run.git.ShortHash(context, run.stableRef)
	}

	title := fmt.Sprintf("%s has %s %s lacks", run.stable, release.Commits(run.lacking), run.branch)
	choices := []ui.Choice{
		{Label: JOIN_MERGE, Detail: run.stable + " + " + run.branch + ", with a merge commit"},
		{Label: JOIN_REBASE, Detail: run.branch + " on top of " + onto + ", rewrites " + run.branch},
	}

	index, menuError := app.UI.Menu(context, title, choices)
	if menuError != nil {
		return "", menuError
	}

	return choices[index].Label, nil
}

// rebase replays the suffix branch on top of the stable branch. A conflict
// aborts it, and nothing has changed.
func (run *stableRun) rebase(context context.Context) error {
	rebaseError := run.git.Rebase(context, run.stableRef)
	if rebaseError != nil {
		run.output.Fail("rebase", run.branch+" onto "+run.stable+"; nothing changed")

		return fmt.Errorf("rebase %s onto %s conflicts; merge instead, or resolve it yourself: %w", run.branch, run.stable, rebaseError)
	}

	run.isRebased = true
	run.output.Done("rebase", run.branch+" ("+release.Commits(run.ahead)+") onto "+run.stable)

	return nil
}

// moveToStable switches to the stable branch and merges the suffix branch:
// a fast-forward after a rebase or when nothing is lacking, else a merge.
func (run *stableRun) moveToStable(context context.Context) error {
	switchError := run.git.Switch(context, run.stable)
	if switchError != nil {
		return fmt.Errorf("switch to %s: %w", run.stable, switchError)
	}

	run.output.Done("switch", run.stable)

	mergeError := run.git.Merge(context, run.branch)
	if mergeError != nil {
		return fmt.Errorf("merge %s: %w", run.branch, mergeError)
	}

	detail := run.branch + " (" + release.Commits(run.ahead) + ")"
	if run.lacking > 0 && !run.isRebased {
		detail += fmt.Sprintf(" + %s's %d", run.stable, run.lacking)
	}

	run.output.Done("merge", detail)

	return nil
}

// undo puts both branches back where they were and switches back to the
// suffix branch. It runs even after Ctrl-C.
func (run *stableRun) undo(parent context.Context, cause error) error {
	detached := withoutCancel(parent)

	var undoErrors []error

	current, _ := run.git.Branch(detached)
	if current == run.stable && run.stableBefore != "" {
		undoErrors = append(undoErrors, run.git.ResetHard(detached, run.stableBefore))
	}

	undoErrors = append(undoErrors, run.git.Switch(detached, run.branch))

	undone := run.stable
	if run.isRebased {
		undoErrors = append(undoErrors, run.git.ResetHard(detached, run.branchBefore))
		undone += " and " + run.branch
	}

	run.output.Fail("switch", run.branch+"; "+undone+" undone")

	return errors.Join(append([]error{cause}, undoErrors...)...)
}

// update switches back to the suffix branch, fast-forwards it to the
// release, and pushes it.
func (run *stableRun) update(context context.Context) error {
	switchError := run.git.Switch(context, run.branch)
	if switchError != nil {
		return fmt.Errorf("switch to %s: %w", run.branch, switchError)
	}

	forwardError := run.git.FastForward(context, run.stable)
	if forwardError != nil {
		return fmt.Errorf("update %s to %s: %w", run.branch, run.stable, forwardError)
	}

	remote, _, isPushed := run.target.Push()
	if isPushed {
		pushError := run.push(context, remote)
		if pushError != nil {
			return pushError
		}
	} else {
		run.output.Done("update", run.branch+" ← "+run.stable)
	}

	run.output.Done("switch", run.branch)

	if run.isRebased {
		run.reportTags(context)
	}

	return nil
}

// push sends the updated suffix branch. After a rebase it is a force-push
// with a lease on what the remote held at the start, so a push someone else
// made since is never lost.
func (run *stableRun) push(context context.Context, remote string) error {
	isForced := run.isRebased && run.remoteBefore != ""

	how := "push to " + remote
	if isForced {
		how = "force-push to " + remote
	}

	task := run.output.Start("update", run.branch+" ← "+run.stable+", "+how)

	var pushError error
	if isForced {
		pushError = run.git.PushForce(context, task, remote, run.branch, run.remoteBefore)
	} else {
		pushError = run.git.PushAtomic(context, task, remote, "refs/heads/"+run.branch)
	}

	if pushError == nil {
		task.Done()

		return nil
	}

	task.Fail()

	if isForced {
		return fmt.Errorf("push %s: %w\n%s changed on %s since final started, so it was kept; the release is out: compare them, then push %s yourself", run.branch, pushError, run.branch, remote, run.branch)
	}

	return fmt.Errorf("push %s: %w", run.branch, pushError)
}

// reportTags names the suffix branch's tags the rebase left behind: they
// point at the old commits, which the branch no longer holds.
func (run *stableRun) reportTags(context context.Context) {
	tags, _ := run.git.Tags(context)

	for _, tag := range tags {
		if !strings.HasSuffix(tag, "-"+run.target.Suffix) {
			continue
		}

		isOld := run.git.IsAncestor(context, tag, run.branchBefore)
		if isOld && !run.git.IsAncestor(context, tag, run.branch) {
			run.output.Skip("tag", tag+" stays on the old "+run.branch)
		}
	}
}

// publishHint says how to finish by hand after a publish failure, which
// leaves the run on the stable branch.
func (run *stableRun) publishHint() string {
	if run.isRebased {
		return fmt.Sprintf("you are on %s: fix it, run the publish steps there, then fast-forward %s to %s and force-push it with a lease", run.stable, run.branch, run.stable)
	}

	return fmt.Sprintf("you are on %s: fix it, run the publish steps there, then merge %s into %s", run.stable, run.stable, run.branch)
}

// withoutCancel keeps the values of parent but not its cancel, so an undo
// still runs after Ctrl-C.
func withoutCancel(parent context.Context) context.Context {
	return context.WithoutCancel(parent)
}
