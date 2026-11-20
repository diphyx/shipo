package semver

import (
	"strconv"
	"strings"
)

// Compare returns -1, 0 or 1, in semver order. Build metadata is ignored.
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

	return comparePrerelease(version.Prerelease(), other.Prerelease())
}

// Prerelease returns the pre-release part without its dash, such as "beta.1",
// or "" for a release.
func (version Version) Prerelease() string {
	prerelease, _, _ := strings.Cut(version.Suffix, "+")

	return strings.TrimPrefix(prerelease, "-")
}

// comparePrerelease orders two pre-release parts as semver does: a release
// sorts after its pre-releases, numbers sort before text, and a longer list of
// equal identifiers sorts later.
func comparePrerelease(left string, right string) int {
	switch {
	case left == right:
		{
			return 0
		}
	case left == "":
		{
			return 1
		}
	case right == "":
		{
			return -1
		}
	}

	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")

	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		order := compareIdentifier(leftParts[index], rightParts[index])
		if order != 0 {
			return order
		}
	}

	return compareInts(len(leftParts), len(rightParts))
}

func compareIdentifier(left string, right string) int {
	leftNumber, leftError := strconv.Atoi(left)
	rightNumber, rightError := strconv.Atoi(right)
	isLeftNumber := leftError == nil
	isRightNumber := rightError == nil

	switch {
	case isLeftNumber && isRightNumber:
		{
			return compareInts(leftNumber, rightNumber)
		}
	case isLeftNumber:
		{
			return -1
		}
	case isRightNumber:
		{
			return 1
		}
	}

	return strings.Compare(left, right)
}

func compareInts(left int, right int) int {
	switch {
	case left < right:
		{
			return -1
		}
	case left > right:
		{
			return 1
		}
	}

	return 0
}
