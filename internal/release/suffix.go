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

	if branch == target.Config.Bump.Stable {
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

// HasSuffix reports whether bump.suffix is set, so versions depend on the branch.
func (target *Target) HasSuffix() bool {
	return target.Config.Bump.Suffix.Enabled()
}

// owns reports whether a version belongs to the current branch: with
// bump.suffix, one with the branch's suffix, or a release on a plain branch.
// Without bump.suffix, every version does.
func (target *Target) owns(version semver.Version) bool {
	if !target.HasSuffix() {
		return true
	}

	return version.Prerelease() == target.Suffix
}
