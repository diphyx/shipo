// Package release ties a .shipo target to its repository and computes the
// values every step shares: versions, tags and keywords.
package release

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/scope/version"
	"github.com/diphyx/shipo/internal/text/keyword"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/tool/git"
	"github.com/diphyx/shipo/internal/tool/github"
	"github.com/diphyx/shipo/internal/tool/process"
)

// Target is a loaded .shipo with its repository.
type Target struct {
	Config  *config.Config
	Runner  process.Runner
	Git     *git.Git
	GitHub  *github.GitHub
	Version *version.Resolved
	// Root is the repository's top folder; Folder is the target relative to it.
	Root   string
	Folder string
	// HeadBranch is the checked-out branch, read only when push or
	// bump.suffix depend on it. Suffix is the pre-release it adds, from
	// bump.suffix, such as "next"; "" on a plain branch.
	HeadBranch string
	Suffix     string

	suffixError error
}

// Open finds the nearest .shipo from dir and loads its target.
func Open(context context.Context, runner process.Runner, dir string) (*Target, error) {
	path, findError := config.Find(dir)
	if findError != nil {
		return nil, findError
	}

	loaded, loadError := config.Load(path)
	if loadError != nil {
		return nil, loadError
	}

	return OpenConfig(context, runner, loaded)
}

// OpenConfig loads the target of an already loaded config.
func OpenConfig(context context.Context, runner process.Runner, loaded *config.Config) (*Target, error) {
	repository := git.NewGit(runner, loaded.Dir)

	root, rootError := repository.Root(context)
	if rootError != nil {
		return nil, rootError
	}

	resolvedRoot, _ := filepath.EvalSymlinks(root)
	resolvedDir, _ := filepath.EvalSymlinks(loaded.Dir)

	folder, relativeError := filepath.Rel(resolvedRoot, resolvedDir)
	if relativeError != nil {
		return nil, relativeError
	}

	branch, branchError := readBranch(context, repository, loaded)
	if branchError != nil {
		return nil, branchError
	}

	pushError := expandPush(context, repository, loaded, branch)
	if pushError != nil {
		return nil, pushError
	}

	resolved, resolveError := version.Resolve(version.Settings{
		Preset:  loaded.Version.Preset,
		Source:  loaded.Version.Source,
		Pattern: loaded.Version.Pattern,
	}, loaded.Dir)
	if resolveError != nil {
		return nil, resolveError
	}

	target := &Target{
		Config:  loaded,
		Runner:  runner,
		Git:     repository,
		GitHub:  github.NewGitHub(runner, loaded.Dir),
		Version: resolved,
		Root:    root,
		Folder:  filepath.ToSlash(folder),

		HeadBranch: branch,
	}

	suffixError := target.loadSuffix(branch)
	if suffixError != nil {
		return nil, suffixError
	}

	return target, nil
}

// ReadCurrent reads the current version.
func (target *Target) ReadCurrent(context context.Context) (*version.Current, error) {
	var tags []string

	if target.Version.IsGit {
		if target.suffixError != nil {
			return nil, target.suffixError
		}

		listed, tagsError := target.Git.Tags(context)
		if tagsError != nil {
			return nil, tagsError
		}

		tags = target.ownTags(listed)
	}

	return version.Read(target.Version, tags)
}

// Release is one release: a target, its old and new versions, and its tag.
type Release struct {
	*Target

	Current     *version.Current
	Level       semver.Level
	Previous    semver.Version
	Next        semver.Version
	Tag         string
	PreviousTag string
	// IsRetag is a final whose tag already exists: like current, it moves
	// the tag to the release commit and sends it out again.
	IsRetag bool
	// Hash is the short hash of the release commit, once it exists.
	Hash string
	Date string
}

// MovesTag reports whether the release reuses an existing tag, moving it to
// HEAD and replacing it on the remote: current, or a final whose tag exists.
func (release *Release) MovesTag() bool {
	return release.Level == semver.LEVEL_CURRENT || release.IsRetag
}

// NewRelease computes the next version and tag for a level.
func NewRelease(context context.Context, target *Target, level semver.Level) (*Release, error) {
	if target.IsStableFinal(level) {
		return nil, fmt.Errorf("final releases on %s: run shipo final, which switches there", target.Config.Bump.Stable)
	}

	current, readError := target.ReadCurrent(context)
	if readError != nil {
		return nil, readError
	}

	next, bumpError := target.Bump(current.Version, level)
	if bumpError != nil {
		return nil, bumpError
	}

	return newRelease(context, target, current, level, next), nil
}

// NewFinal plans final on the stable branch for a pre-release made on a
// suffix branch, as 1.2.0-next to 1.2.0. The pre-release is read before the
// switch: with git tags, the stable branch only sees its own releases.
func NewFinal(context context.Context, target *Target, from semver.Version) (*Release, error) {
	current, readError := target.ReadCurrent(context)
	if readError != nil {
		return nil, readError
	}

	next, _ := from.Bump(semver.LEVEL_FINAL)
	release := newRelease(context, target, current, semver.LEVEL_FINAL, next)

	if target.owns(current.Version) && next.Compare(current.Version) < 0 {
		return nil, fmt.Errorf("final: %s is below %s, the last release on %s; bump the pre-release first", next, current.Version, target.Config.Bump.Stable)
	}

	return release, nil
}

// newRelease builds a release from the current version to the next.
func newRelease(context context.Context, target *Target, current *version.Current, level semver.Level, next semver.Version) *Release {
	previousTag := target.Version.TagName(target.Folder, current.Version.String())
	if !target.Git.HasTag(context, previousTag) {
		previousTag = ""
	}

	release := &Release{
		Target:      target,
		Current:     current,
		Level:       level,
		Previous:    current.Version,
		Next:        next,
		Tag:         target.Version.TagName(target.Folder, next.String()),
		PreviousTag: previousTag,
		Date:        time.Now().Format("2006-01-02"),
	}

	// A final whose tag is out already is released again, as current is.
	release.IsRetag = level == semver.LEVEL_FINAL && target.Git.HasTag(context, release.Tag)

	// The current version can be another branch's: 1.3.0-next on main after
	// a merge, or 1.2.3 on a new next; or a retag's own. The previous tag is
	// then this branch's last one below, or the release it branched from.
	if !target.owns(current.Version) || release.PreviousTag == release.Tag {
		found, _ := release.findPreviousTag(context)
		if found != "" {
			release.PreviousTag = found
		}
	}

	return release
}

// ErrNoRelease is returned when HEAD isn't a release commit with its tag.
var ErrNoRelease = errors.New("HEAD is not a release: run `shipo bump <level>` first")

// FromHead loads the release that HEAD already is: the current version, with
// its tag pointing at HEAD.
func FromHead(context context.Context, target *Target) (*Release, error) {
	release, planError := NewRelease(context, target, semver.LEVEL_CURRENT)
	if planError != nil {
		return nil, planError
	}

	tags, tagsError := target.Git.TagsAt(context, "HEAD")
	if tagsError != nil {
		return nil, tagsError
	}

	isTagged := false

	for _, tag := range tags {
		if tag == release.Tag {
			isTagged = true
		}
	}

	if !isTagged {
		return nil, fmt.Errorf("%w (tag %s is not on HEAD)", ErrNoRelease, release.Tag)
	}

	release.PreviousTag, release.Previous = release.findPreviousTag(context)
	release.LoadHash(context)

	return release, nil
}

// LoadHash reads the short hash of HEAD into the release.
func (release *Release) LoadHash(context context.Context) {
	hash, hashError := release.Git.ShortHash(context, "HEAD")
	if hashError == nil {
		release.Hash = hash
	}
}

// Values returns the shared keywords.
func (release *Release) Values() keyword.Values {
	remote, branch, _ := release.Push()

	values := keyword.Values{
		"version":      release.Next.String(),
		"previous":     release.Previous.String(),
		"major":        fmt.Sprint(release.Next.Major),
		"minor":        fmt.Sprint(release.Next.Minor),
		"patch":        fmt.Sprint(release.Next.Patch),
		"tag":          release.Tag,
		"previous_tag": release.PreviousTag,
		"bump":         string(release.Level),
		"date":         release.Date,
		"branch":       branch,
		"remote":       remote,
		"target":       release.Folder,
	}

	if release.Hash != "" {
		values["hash"] = release.Hash
	}

	return values
}
