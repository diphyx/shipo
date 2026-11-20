package release

import (
	"context"

	"github.com/diphyx/shipo/internal/scope/version"
	"github.com/diphyx/shipo/internal/text/semver"
)

// findPreviousTag returns the highest tag below the release. With
// bump.suffix it is the branch's own; a suffix branch with none yet falls
// back to the releases it branched from.
func (release *Release) findPreviousTag(context context.Context) (string, semver.Version) {
	tags, _ := release.Git.Tags(context)

	best, bestVersion := release.highestBelow(tags, release.owns)
	if best == "" && release.Suffix != "" {
		best, bestVersion = release.highestBelow(tags, func(version semver.Version) bool {
			return version.Prerelease() == ""
		})
	}

	return best, bestVersion
}

func (release *Release) highestBelow(tags []string, isWanted func(semver.Version) bool) (string, semver.Version) {
	best := ""
	bestVersion := semver.Version{}

	for _, tag := range tags {
		parsed, isMatch := release.parseTag(tag)
		if !isMatch || !isWanted(parsed) || parsed.Compare(release.Next) >= 0 {
			continue
		}

		if best == "" || parsed.Compare(bestVersion) > 0 {
			best = tag
			bestVersion = parsed
		}
	}

	return best, bestVersion
}

// ownTags keeps the tags of this target that belong to the current branch.
func (target *Target) ownTags(tags []string) []string {
	var own []string

	for _, tag := range tags {
		parsed, isMatch := target.parseTag(tag)
		if isMatch && target.owns(parsed) {
			own = append(own, tag)
		}
	}

	return own
}

// parseTag reads the version of one of this target's tags.
func (target *Target) parseTag(tag string) (semver.Version, bool) {
	prefix := target.Version.TagName(target.Folder, "")

	if target.Version.IsGit {
		pattern, compileError := version.ParseTag(target.Version.Patterns[0])
		if compileError != nil {
			return semver.Version{}, false
		}

		match, isMatch := pattern.MatchFull(tag)
		if !isMatch {
			return semver.Version{}, false
		}

		parsed, parseError := semver.Parse(match.Version)

		return parsed, parseError == nil
	}

	if len(tag) <= len(prefix) || tag[:len(prefix)] != prefix {
		return semver.Version{}, false
	}

	parsed, parseError := semver.Parse(tag[len(prefix):])

	return parsed, parseError == nil
}
