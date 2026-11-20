package check

import (
	"fmt"
	"slices"
	"strings"

	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/text/semver"
)

// suffixNames are the pre-release names a stack accepts, for the stacks
// that don't take any: PEP 440 for Python, Composer's stabilities for PHP.
var suffixNames = map[string][]string{
	"python": {"a", "b", "c", "rc", "alpha", "beta", "pre", "preview", "dev", "post"},
	"php":    {"a", "b", "p", "rc", "alpha", "beta", "patch", "pl", "dev"},
}

// suffix checks a suffix branch: the stack must accept its name, and it
// must push to itself, since its versions don't belong on another branch.
func suffix(plan *release.Release) []Item {
	target := plan.Target
	if target.Suffix == "" || plan.Level == semver.LEVEL_CURRENT {
		return nil
	}

	var items []Item

	names, isLimited := suffixNames[target.Version.Preset]
	if isLimited && !slices.ContainsFunc(names, func(name string) bool { return strings.EqualFold(name, target.Suffix) }) {
		items = append(items, Item{Name: "suffix", Detail: fmt.Sprintf("%s versions can't end in -%s; use a branch named alpha, beta or rc", target.Version.Preset, target.Suffix)})
	}

	for _, entry := range target.Config.Publish.Push.Items {
		_, branch, _ := config.SplitPush(entry)
		if branch != target.HeadBranch {
			items = append(items, Item{Name: "suffix", Detail: fmt.Sprintf("%s would get %s from %s; push to %s, as push = \"{remote}/{branch}\"", entry, plan.Tag, target.HeadBranch, target.HeadBranch)})
		}
	}

	if len(items) == 0 {
		items = append(items, Item{Name: "suffix", IsOK: true, Detail: "-" + target.Suffix + " on " + target.HeadBranch})
	}

	return items
}
