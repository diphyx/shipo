package release

import (
	"fmt"

	"github.com/diphyx/shipo/internal/text/semver"
)

// Levels returns the levels that fit the current version on this branch, in
// menu order.
func (target *Target) Levels(current semver.Version) []semver.Level {
	var levels []semver.Level

	for _, level := range semver.Levels {
		if target.allows(current, level) == nil {
			levels = append(levels, level)
		}
	}

	return levels
}

// allows says whether a level fits the current version on this branch:
// current needs the branch's own version, so it can't reuse another
// branch's tag. final needs a pre-release: another branch's on a plain one,
// or on a suffix branch its own, released on the stable branch.
func (target *Target) allows(current semver.Version, level semver.Level) error {
	if target.suffixError != nil {
		if level == semver.LEVEL_CURRENT {
			return nil
		}

		return target.suffixError
	}

	isOwn := target.owns(current)

	switch level {
	case semver.LEVEL_CURRENT:
		{
			if !isOwn && target.Suffix == "" {
				return fmt.Errorf("current: %s is a pre-release from another branch; use final", current)
			}

			if !isOwn {
				return fmt.Errorf("current: %s is not a -%s version; bump it with patch, minor or major", current, target.Suffix)
			}
		}
	case semver.LEVEL_FINAL:
		{
			if target.Suffix != "" && !isOwn {
				return fmt.Errorf("final: %s is not a -%s version", current, target.Suffix)
			}

			if target.Suffix == "" && isOwn {
				return fmt.Errorf("final: %s is not a pre-release from another branch", current)
			}
		}
	}

	return nil
}

// Bump returns the next version for a level. On a suffix branch, it ends in
// the branch's name, as 1.3.0-next.
func (target *Target) Bump(current semver.Version, level semver.Level) (semver.Version, error) {
	allowError := target.allows(current, level)
	if allowError != nil {
		return semver.Version{}, allowError
	}

	next, bumpError := current.Bump(level)
	if bumpError != nil {
		return semver.Version{}, bumpError
	}

	if target.Suffix != "" && level != semver.LEVEL_CURRENT && level != semver.LEVEL_FINAL {
		next.Suffix = "-" + target.Suffix
	}

	return next, nil
}

// IsStableFinal reports whether a level is final on a suffix branch, which
// releases on the stable branch instead of here.
func (target *Target) IsStableFinal(level semver.Level) bool {
	return level == semver.LEVEL_FINAL && target.Suffix != ""
}
