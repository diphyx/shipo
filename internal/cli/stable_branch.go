package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/tool/git"
	"github.com/diphyx/shipo/internal/ui"
)

// stableRun moves between the suffix branch and the stable branch for one
// final, and keeps where both were, to undo.
type stableRun struct {
	target *release.Target
	git    *git.Git
	output *ui.Terminal
	branch string
	stable string
	// source is what final releases: the commit of the suffix branch's last
	// pre-release tag, what was tested, or the branch itself with none.
	// sourceName names it: the branch, or the tag when the branch moved on
	// past it. from is its version.
	source     string
	sourceName string
	from       semver.Version
	// stableBefore is the stable branch's commit at the start; "" when it
	// isn't local yet.
	stableBefore string
	// stableRef is the stable branch to count the suffix branch against: the
	// local one, or the remote's when it is ahead or the local one isn't
	// there yet.
	stableRef string
	// missing lists the commits of the stable branch, local or remote, that
	// the source lacks: what merge brings in, and replace drops.
	missing []string
	// stableRemote is the stable branch on its remote: the lease when
	// replace drops its commits.
	stableRemote string
	// ahead counts the source's commits the stable branch lacks; lacking,
	// the stable branch's commits the source lacks.
	ahead   int
	lacking int
	// canMerge is false when the source and the stable branch conflict.
	canMerge   bool
	isReplaced bool
	// backups are the branches backup saved the stable branch in.
	backups []string
}

// newStableRun checks the suffix branch can go to the stable branch, with a
// clean tree, picks what final releases, and reads where both stand. pre is
// the suffix branch's version, for when it has no pre-release tag yet.
func newStableRun(context context.Context, target *release.Target, output *ui.Terminal, pre semver.Version) (*stableRun, error) {
	run := &stableRun{
		target:     target,
		git:        target.Git,
		output:     output,
		branch:     target.HeadBranch,
		stable:     target.Config.Bump.Stable,
		source:     target.HeadBranch,
		sourceName: target.HeadBranch,
		from:       pre,
		canMerge:   true,
	}

	isClean, cleanError := run.git.IsClean(context)
	if cleanError != nil {
		return nil, cleanError
	}

	if !isClean {
		return nil, fmt.Errorf("the tree has changes; commit or stash them before final switches to %s", run.stable)
	}

	// Final releases what was tested: the last pre-release tag, even when
	// the branch moved on past it.
	preview, hasPreview := target.LastPreview(context)
	if hasPreview {
		run.source = preview.Commit
		run.from = preview.Version

		if preview.Commit != run.git.Resolve(context, run.branch) {
			run.sourceName = preview.Tag
		}
	}

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

	// Both the local and the remote stable branch count: when they diverged,
	// replace drops the commits of each.
	var stableRefs []string
	if run.stableBefore != "" {
		stableRefs = append(stableRefs, run.stable)
	}

	if run.stableRemote != "" {
		stableRefs = append(stableRefs, remote+"/"+run.stable)
	}

	run.ahead, _ = run.git.CountCommits(context, run.stableRef, run.source)
	run.missing, _ = run.git.Log(context, run.source, stableRefs...)
	run.lacking = len(run.missing)

	for _, ref := range stableRefs {
		if !run.git.CanMergeAt(context, run.source, ref) {
			run.canMerge = false
		}
	}

	return run, nil
}

// readRemote fetches, so commits pushed to either branch since the last
// fetch count. The suffix branch must hold all of its remote's commits. The
// stable branch is compared from its remote when the local one is behind
// it, or not there yet.
func (run *stableRun) readRemote(context context.Context, remote string) error {
	fetchError := run.git.Fetch(context, remote)
	if fetchError != nil {
		return fmt.Errorf("git fetch %s: %w", remote, fetchError)
	}

	remoteBranch := remote + "/" + run.branch
	if run.git.Resolve(context, remoteBranch) != "" && !run.git.IsAncestor(context, remoteBranch, run.branch) {
		return fmt.Errorf("%s has commits you don't have; pull them into %s first", remoteBranch, run.branch)
	}

	remoteStable := remote + "/" + run.stable
	run.stableRemote = run.git.Resolve(context, remoteStable)

	if run.stableRemote != "" && (run.stableBefore == "" || run.git.IsAncestor(context, run.stable, remoteStable)) {
		run.stableRef = remoteStable
	}

	return nil
}

// replaces is the remote commit the release's push replaces, for replace;
// "" for a merge, which pushes as usual.
func (run *stableRun) replaces() string {
	if !run.isReplaced {
		return ""
	}

	return run.stableRemote
}

// undo puts the stable branch back where it was, or removes it when the
// switch made it, and switches back to the suffix branch, which nothing
// changed. It runs even after Ctrl-C.
func (run *stableRun) undo(parent context.Context, cause error) error {
	detached := withoutCancel(parent)

	var undoErrors []error

	// A reset also clears a merge that stopped halfway, so the switch back
	// can go through.
	current, _ := run.git.Branch(detached)
	if current == run.stable {
		back := run.stableBefore
		if back == "" {
			back = "HEAD"
		}

		undoErrors = append(undoErrors, run.git.ResetHard(detached, back))
	}

	undoErrors = append(undoErrors, run.git.Switch(detached, run.branch))

	if run.stableBefore == "" && run.git.Resolve(detached, run.stable) != "" {
		undoErrors = append(undoErrors, run.git.DeleteBranch(detached, run.stable))
	}

	run.output.Fail("switch", run.branch+"; "+run.stable+" undone")

	return errors.Join(append([]error{cause}, undoErrors...)...)
}

// update switches back to the suffix branch and brings the release in: a
// fast-forward when the branch is what was released, else a merge, as it
// moved on past its tag. It is never rewritten. A merge that would conflict
// is left to the user: the release is out already.
func (run *stableRun) update(context context.Context) error {
	switchError := run.git.Switch(context, run.branch)
	if switchError != nil {
		return fmt.Errorf("switch to %s: %w", run.branch, switchError)
	}

	how := ""

	if run.git.FastForward(context, run.stable) != nil {
		if !run.git.CanMerge(context, run.stable) {
			run.output.Skip("update", run.branch+" conflicts with "+run.stable+"; merge it in when ready")
			run.output.Done("switch", run.branch)

			return nil
		}

		mergeError := run.git.Merge(context, run.stable)
		if mergeError != nil {
			return fmt.Errorf("merge %s into %s: %w", run.stable, run.branch, mergeError)
		}

		how = ", merged"
	}

	remote, _, isPushed := run.target.Push()
	if !isPushed {
		run.output.Done("update", run.branch+" ← "+run.stable+how)
		run.output.Done("switch", run.branch)

		return nil
	}

	task := run.output.Start("update", run.branch+" ← "+run.stable+how+", push to "+remote)

	pushError := run.git.PushAtomic(context, task, remote, "refs/heads/"+run.branch)
	if pushError != nil {
		task.Fail()

		return fmt.Errorf("push %s: %w", run.branch, pushError)
	}

	task.Done()
	run.output.Done("switch", run.branch)

	return nil
}

// publishHint says how to finish by hand after a publish failure, which
// leaves the run on the stable branch. A replace can't be pushed by shipo
// push, a plain push: it gets the push with its lease.
func (run *stableRun) publishHint(plan *release.Release) string {
	remote, _, _ := plan.Push()

	if run.isReplaced && run.stableRemote != "" {
		return fmt.Sprintf("you are on %s, replaced by %s; once the cause is fixed, push it with its lease:\n  git push --atomic --force-with-lease=refs/heads/%s:%s %s %s refs/tags/%s\nthen run the other publish steps there, and merge %s into %s", run.stable, run.sourceName, run.stable, run.stableRemote, remote, run.stable, plan.Tag, run.stable, run.branch)
	}

	return fmt.Sprintf("you are on %s: fix it, run the publish steps there, then merge %s into %s", run.stable, run.stable, run.branch)
}

// withoutCancel keeps the values of parent but not its cancel, so an undo
// still runs after Ctrl-C.
func withoutCancel(parent context.Context) context.Context {
	return context.WithoutCancel(parent)
}
