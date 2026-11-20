package release

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/diphyx/shipo/internal/text/semver"
)

// suffixPattern is what a branch name may hold once it is a pre-release.
var suffixPattern = regexp.MustCompile(`^[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*$`)

// loadSuffix sets the pre-release the checked-out branch adds. A detached
// HEAD is only an error once a version must be made or tags read.
func (target *Target) loadSuffix(branch string) error {
	suffix := target.Config.Bump.Suffix
	if !suffix.Enabled() {
		return nil
	}

	if branch == "" {
		target.suffixError = errors.New("bump.suffix needs a checked-out branch, HEAD is detached")

		return nil
	}

	for _, pattern := range suffix.Items {
		isMatch, _ := path.Match(pattern, branch)
		if !isMatch {
			continue
		}

		name := strings.ReplaceAll(branch, "/", "-")
		if !suffixPattern.MatchString(name) {
			return fmt.Errorf("bump.suffix: branch %s can't be a version suffix", branch)
		}

		target.Suffix = name

		break
	}

	return nil
}

// hasSuffix reports whether bump.suffix is set, so versions depend on the branch.
func (target *Target) hasSuffix() bool {
	return target.Config.Bump.Suffix.Enabled()
}

// Bump returns the next version for a level. With bump.suffix, a suffix
// branch adds its name, as 1.3.0-next, and a plain branch promotes a
// pre-release, as 1.3.0-next to 1.3.0.
func (target *Target) Bump(current semver.Version, level semver.Level) (semver.Version, error) {
	if target.suffixError != nil && level != semver.LEVEL_CURRENT {
		return semver.Version{}, target.suffixError
	}

	if !target.hasSuffix() || level == semver.LEVEL_CURRENT {
		return current.Bump(level)
	}

	if target.Suffix == "" {
		return current.Promote(level)
	}

	next, bumpError := current.Bump(level)
	next.Suffix = "-" + target.Suffix

	return next, bumpError
}

// isOwnTag reports whether a tag's version belongs to the current branch:
// with bump.suffix, one with its suffix, or a release on a plain branch.
func (target *Target) isOwnTag(version semver.Version) bool {
	if !target.hasSuffix() {
		return true
	}

	return version.Prerelease() == target.Suffix
}
