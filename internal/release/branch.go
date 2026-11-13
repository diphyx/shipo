package release

import (
	"context"
	"fmt"

	"github.com/diphyx/shipo/internal/config"
)

// Branch is where HEAD stands against a push branch, as last fetched.
type Branch struct {
	// Entry is the push entry, "remote/branch".
	Entry string
	// Revision is the branch's commit, or "" when it doesn't exist yet.
	Revision string
	// Behind counts the branch's commits that HEAD lacks. CanMerge is true
	// when they merge into HEAD with no conflict.
	Behind   int
	CanMerge bool
}

// NewBranch reads where HEAD stands against a push entry.
func NewBranch(context context.Context, target *Target, entry string) *Branch {
	remote, name, _ := config.SplitPush(entry)
	branch := &Branch{Entry: entry, Revision: target.Git.Resolve(context, remote+"/"+name)}

	if branch.Revision == "" || target.Git.IsAncestor(context, branch.Revision, "HEAD") {
		return branch
	}

	branch.Behind, _ = target.Git.CountCommits(context, "HEAD", branch.Revision)
	branch.Behind = max(branch.Behind, 1)
	branch.CanMerge = target.Git.CanMerge(context, branch.Revision)

	return branch
}

// Commits says how many commits HEAD lacks: "1 commit", "3 commits".
func (branch *Branch) Commits() string {
	if branch.Behind == 1 {
		return "1 commit"
	}

	return fmt.Sprintf("%d commits", branch.Behind)
}
