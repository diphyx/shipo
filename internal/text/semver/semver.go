// Package semver parses and bumps MAJOR.MINOR.PATCH versions.
package semver

import (
	"fmt"
	"regexp"
	"strconv"
)

// PATTERN matches a version, with an optional pre-release and build suffix.
const PATTERN = `\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`

var fullPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// Level is how a version is bumped.
type Level string

// The bump levels.
const (
	LEVEL_ASK     Level = "ask"
	LEVEL_MAJOR   Level = "major"
	LEVEL_MINOR   Level = "minor"
	LEVEL_PATCH   Level = "patch"
	LEVEL_CURRENT Level = "current"
)

// Levels lists the levels that pick a version, in menu order.
var Levels = []Level{LEVEL_PATCH, LEVEL_MINOR, LEVEL_MAJOR, LEVEL_CURRENT}

// ParseLevel validates a level name.
func ParseLevel(name string) (Level, error) {
	level := Level(name)

	switch level {
	case LEVEL_ASK, LEVEL_MAJOR, LEVEL_MINOR, LEVEL_PATCH, LEVEL_CURRENT:
		{
			return level, nil
		}
	}

	return "", fmt.Errorf("unknown level %q: want ask, major, minor, patch or current", name)
}

// Version is a parsed semantic version.
type Version struct {
	Major, Minor, Patch int
	// Suffix is the pre-release and build part, such as "-beta.1+5".
	Suffix string
}

// ZERO is the start when there is no version yet.
var ZERO = Version{}

// Parse reads a version such as 1.2.3 or 1.2.3-beta.1.
func Parse(text string) (Version, error) {
	match := fullPattern.FindStringSubmatch(text)
	if match == nil {
		return Version{}, fmt.Errorf("%q is not a MAJOR.MINOR.PATCH version", text)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])

	return Version{Major: major, Minor: minor, Patch: patch, Suffix: match[4] + match[5]}, nil
}

// String formats the version.
func (version Version) String() string {
	return fmt.Sprintf("%d.%d.%d%s", version.Major, version.Minor, version.Patch, version.Suffix)
}

// Bump returns the next version for a level. A suffix is dropped.
func (version Version) Bump(level Level) (Version, error) {
	switch level {
	case LEVEL_MAJOR:
		{
			return Version{Major: version.Major + 1}, nil
		}
	case LEVEL_MINOR:
		{
			return Version{Major: version.Major, Minor: version.Minor + 1}, nil
		}
	case LEVEL_PATCH:
		{
			return Version{Major: version.Major, Minor: version.Minor, Patch: version.Patch + 1}, nil
		}
	case LEVEL_CURRENT:
		{
			return version, nil
		}
	}

	return Version{}, fmt.Errorf("cannot bump with level %q", level)
}

// Promote returns the release a pre-release leads to, as npm does: 1.3.0-next
// with minor gives 1.3.0, and 1.3.1-next with minor gives 1.4.0. A release is
// bumped as usual.
func (version Version) Promote(level Level) (Version, error) {
	if version.Prerelease() == "" {
		return version.Bump(level)
	}

	release := Version{Major: version.Major, Minor: version.Minor, Patch: version.Patch}

	switch level {
	case LEVEL_MAJOR:
		{
			if version.Minor == 0 && version.Patch == 0 {
				return release, nil
			}
		}
	case LEVEL_MINOR:
		{
			if version.Patch == 0 {
				return release, nil
			}
		}
	case LEVEL_PATCH:
		{
			return release, nil
		}
	}

	return version.Bump(level)
}
