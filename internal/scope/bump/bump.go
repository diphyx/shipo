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
}

// Revert undoes a bump: it deletes the tag and resets to the base commit.
// The working tree was clean before the bump, so nothing of the user's is lost.
func Revert(context context.Context, target *release.Target, result *Result) error {
	var revertErrors []error

	if result.IsTagged {
		revertErrors = append(revertErrors, target.Git.DeleteTag(context, result.Tag))
	}

	if result.Base != "" {
		revertErrors = append(revertErrors, target.Git.ResetHard(context, result.Base))
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
			lockError := plan.Runner.Run(context, lock.Command)
			if lockError != nil {
				return fmt.Errorf("refresh %s: %w", plan.Config.Rel(lock.File), lockError)
			}

			changed = append(changed, lock.File)
			output.Done("lock", plan.Config.Rel(lock.File))
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
