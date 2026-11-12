// Package bump writes the next version, refreshes lock files, commits and
// tags. Everything here is local and can be reverted.
package bump

import (
	"context"
	"errors"
	"fmt"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/scope/version"
	"github.com/diphyx/shipo/internal/text/keyword"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/ui"
)

// Result is a finished bump, with what is needed to revert it.
type Result struct {
	// Base is the commit HEAD was on before the bump.
	Base string
	// IsCommitted and IsTagged report what the bump created.
	IsCommitted bool
	IsTagged    bool
	Tag         string
	// MovedFrom is the commit the tag was on, when current moved it to HEAD.
	MovedFrom string
}

// Revert undoes a bump: it deletes the tag and resets to the base commit.
// The working tree was clean before the bump, so nothing of the user's is lost.
// It runs even when the context is cancelled, so Ctrl-C can't stop it halfway.
func Revert(parent context.Context, target *release.Target, result *Result) error {
	detached := context.WithoutCancel(parent)

	var revertErrors []error

	if result.IsTagged {
		revertErrors = append(revertErrors, target.Git.DeleteTag(detached, result.Tag))
	}

	if result.MovedFrom != "" {
		revertErrors = append(revertErrors, target.Git.TagAt(detached, result.Tag, result.MovedFrom, "Release "+result.Tag))
	}

	if result.Base != "" {
		revertErrors = append(revertErrors, target.Git.ResetHard(detached, result.Base))
	}

	return errors.Join(revertErrors...)
}

// Create bumps a planned release: it writes, commits and tags. On failure it
// reverts what it did.
func Create(context context.Context, plan *release.Release, output *ui.Terminal) (*Result, error) {
	base, headError := plan.Git.Head(context)
	if headError != nil {
		return nil, headError
	}

	result := &Result{Base: base, Tag: plan.Tag}

	if plan.Level == semver.LEVEL_CURRENT {
		output.Skip("version", plan.Next.String()+" (current)")

		tagError := retag(context, plan, output, result)
		if tagError != nil {
			revertError := Revert(context, plan.Target, result)

			return nil, errors.Join(tagError, revertError)
		}

		plan.LoadHash(context)

		return result, nil
	}

	runError := run(context, plan, output, result)
	if runError != nil {
		revertError := Revert(context, plan.Target, result)

		return nil, errors.Join(runError, revertError)
	}

	plan.LoadHash(context)

	return result, nil
}

func run(context context.Context, plan *release.Release, output *ui.Terminal, result *Result) error {
	shortHash, _ := plan.Git.ShortHash(context, "HEAD")

	var changed []string

	if !plan.Version.IsGit {
		written, writeError := plan.Current.Write(version.Fill{Version: plan.Next.String(), Hash: shortHash, Date: plan.Date})
		if writeError != nil {
			return writeError
		}

		changed = append(changed, written...)

		for _, file := range plan.Current.Files() {
			output.Done("version", fmt.Sprintf("%s → %s", plan.Config.Rel(file), plan.Next))
		}

		locks, locksError := plan.Version.Locks()
		if locksError != nil {
			return locksError
		}

		for _, lock := range locks {
			task := output.Start("lock", plan.Config.Rel(lock.File))
			command := lock.Command
			command.Output = task

			lockError := plan.Runner.Run(context, command)
			if lockError != nil {
				task.Fail()

				return fmt.Errorf("refresh %s: %w", plan.Config.Rel(lock.File), lockError)
			}

			changed = append(changed, lock.File)
			task.Done()
		}
	} else {
		output.Done("version", fmt.Sprintf("%s → %s (git tag)", plan.Previous, plan.Next))
	}

	values := plan.Values()
	delete(values, "hash")
	message := keyword.Expand(plan.Config.Bump.Commit, values)

	if len(changed) > 0 {
		addError := plan.Git.Add(context, changed...)
		if addError != nil {
			return addError
		}

		commitError := plan.Git.Commit(context, message, false)
		if commitError != nil {
			return commitError
		}

		result.IsCommitted = true
		output.Done("commit", message)
	}

	tagError := plan.Git.Tag(context, plan.Tag, "Release "+plan.Tag)
	if tagError != nil {
		return tagError
	}

	result.IsTagged = true
	output.Done("tag", plan.Tag)

	return nil
}

// retag puts the current version's tag on HEAD, so the same version can be
// released again from a newer commit. A tag already on HEAD is kept; a tag on
// another commit is moved, and Revert puts it back.
func retag(context context.Context, plan *release.Release, output *ui.Terminal, result *Result) error {
	tagged := plan.Git.Resolve(context, plan.Tag)

	if tagged == result.Base {
		output.Skip("tag", plan.Tag+" (on HEAD)")

		return nil
	}

	detail := plan.Tag

	if tagged != "" {
		deleteError := plan.Git.DeleteTag(context, plan.Tag)
		if deleteError != nil {
			return deleteError
		}

		result.MovedFrom = tagged
		detail += " moved to HEAD"
	}

	tagError := plan.Git.Tag(context, plan.Tag, "Release "+plan.Tag)
	if tagError != nil {
		return tagError
	}

	result.IsTagged = true
	output.Done("tag", detail)

	return nil
}
