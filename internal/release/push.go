package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/text/keyword"
	"github.com/diphyx/shipo/internal/tool/git"
)

// readBranch reads the checked-out branch when publish.push or bump.suffix
// depend on it, and "" otherwise or when HEAD is detached.
func readBranch(context context.Context, repository *git.Git, loaded *config.Config) (string, error) {
	if !hasPushKeyword(loaded) && !loaded.Bump.Suffix.Enabled() {
		return "", nil
	}

	return repository.Branch(context)
}

func hasPushKeyword(loaded *config.Config) bool {
	for _, entry := range loaded.Publish.Push.Items {
		if strings.ContainsAny(entry, "{}") {
			return true
		}
	}

	return false
}

// expandPush fills {branch} and {remote} in publish.push with the current
// branch and the remote it tracks, and rejects any other {name}.
func expandPush(context context.Context, repository *git.Git, loaded *config.Config, branch string) error {
	if !hasPushKeyword(loaded) {
		return nil
	}

	push := &loaded.Publish.Push

	values := keyword.Values{}
	if branch != "" {
		values["branch"] = branch

		remote := repository.UpstreamRemote(context, branch)
		if remote != "" {
			values["remote"] = remote
		}
	}

	for index, entry := range push.Items {
		expanded := keyword.Expand(entry, values)

		if strings.ContainsAny(expanded, "{}") {
			return fmt.Errorf("publish.push: %q: %w", entry, pushKeywordError(entry, branch, values))
		}

		_, _, splitError := config.SplitPush(expanded)
		if splitError != nil {
			return fmt.Errorf("publish.push: %w", splitError)
		}

		push.Items[index] = expanded
	}

	return nil
}

func pushKeywordError(entry string, branch string, values keyword.Values) error {
	switch {
	case branch == "" && (strings.Contains(entry, "{branch}") || strings.Contains(entry, "{remote}")):
		{
			return errors.New("{branch} and {remote} need a checked-out branch, HEAD is detached")
		}
	case strings.Contains(entry, "{remote}") && values["remote"] == "":
		{
			return fmt.Errorf("{remote} needs an upstream; %s tracks no remote (git push -u)", branch)
		}
	default:
		{
			return errors.New("only {branch} and {remote} work in push")
		}
	}
}

// Push returns the main push remote and branch, or false when push is off.
func (target *Target) Push() (string, string, bool) {
	push := target.Config.Publish.Push
	if !push.Enabled() {
		return "", "", false
	}

	remote, branch, _ := config.SplitPush(push.Items[0])

	return remote, branch, true
}
