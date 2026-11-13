package bump

import (
	"context"
	"fmt"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/ui"
)

// merge brings in the commits of the push branches that HEAD lacks, when they
// merge with no conflict, as the checks planned. Revert resets the merge
// with the bump.
func merge(context context.Context, plan *release.Release, output *ui.Terminal) error {
	if !plan.Config.Publish.Push.Enabled() {
		return nil
	}

	isMerged := false

	for _, entry := range plan.Config.Publish.Push.Items {
		branch := release.NewBranch(context, plan.Target, entry)
		if branch.Behind == 0 {
			continue
		}

		if !branch.CanMerge {
			return fmt.Errorf("%s has commits that conflict with HEAD; pull and resolve them first", entry)
		}

		mergeError := plan.Git.Merge(context, branch.Revision)
		if mergeError != nil {
			return fmt.Errorf("merge %s: %w", entry, mergeError)
		}

		isMerged = true
		output.Done("merge", entry+" ("+branch.Commits()+")")
	}

	if !isMerged {
		return nil
	}

	return replan(context, plan)
}

// replan reads the plan again after a merge, and stops when the merged
// commits changed the version the plan was made from.
func replan(context context.Context, plan *release.Release) error {
	fresh, planError := release.NewRelease(context, plan.Target, plan.Level)
	if planError != nil {
		return planError
	}

	if fresh.Previous.Compare(plan.Previous) != 0 {
		return fmt.Errorf("the merged commits change the version from %s to %s; run shipo again", plan.Previous, fresh.Previous)
	}

	*plan = *fresh

	return nil
}
