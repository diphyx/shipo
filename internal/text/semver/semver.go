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

// Compare returns -1, 0 or 1. A pre-release sorts before its release.
func (version Version) Compare(other Version) int {
	pairs := [][2]int{
		{version.Major, other.Major},
		{version.Minor, other.Minor},
		{version.Patch, other.Patch},
	}

	for _, pair := range pairs {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}

			return 1
		}
	}

	isPre := version.Suffix != "" && version.Suffix[0] == '-'
	isOtherPre := other.Suffix != "" && other.Suffix[0] == '-'

	switch {
	case isPre && !isOtherPre:
		{
			return -1
		}
	case !isPre && isOtherPre:
		{
			return 1
		}
	}

	return 0
}
