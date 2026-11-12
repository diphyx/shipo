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

	resolved, resolveError := version.Resolve(version.Settings{
		Preset:  loaded.Version.Preset,
		Source:  loaded.Version.Source,
		Pattern: loaded.Version.Pattern,
	}, loaded.Dir)
	if resolveError != nil {
		return nil, resolveError
	}

	return &Target{
		Config:  loaded,
		Runner:  runner,
		Git:     repository,
		GitHub:  github.NewGitHub(runner, loaded.Dir),
		Version: resolved,
		Root:    root,
		Folder:  filepath.ToSlash(folder),
	}, nil
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

// ReadCurrent reads the current version.
func (target *Target) ReadCurrent(context context.Context) (*version.Current, error) {
	var tags []string

	if target.Version.IsGit {
		listed, tagsError := target.Git.Tags(context)
		if tagsError != nil {
			return nil, tagsError
		}

		tags = listed
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
	// Hash is the short hash of the release commit, once it exists.
	Hash string
	Date string
}

// NewRelease computes the next version and tag for a level.
func NewRelease(context context.Context, target *Target, level semver.Level) (*Release, error) {
	current, readError := target.ReadCurrent(context)
	if readError != nil {
		return nil, readError
	}

	next, bumpError := current.Version.Bump(level)
	if bumpError != nil {
		return nil, bumpError
	}

	previousTag := target.Version.TagName(target.Folder, current.Version.String())
	if !target.Git.HasTag(context, previousTag) {
		previousTag = ""
	}

	return &Release{
		Target:      target,
		Current:     current,
		Level:       level,
		Previous:    current.Version,
		Next:        next,
		Tag:         target.Version.TagName(target.Folder, next.String()),
		PreviousTag: previousTag,
		Date:        time.Now().Format("2006-01-02"),
	}, nil
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

func (release *Release) findPreviousTag(context context.Context) (string, semver.Version) {
	tags, _ := release.Git.Tags(context)

	best := ""
	bestVersion := semver.Version{}

	for _, tag := range tags {
		parsed, isMatch := release.parseTag(tag)
		if !isMatch || parsed.Compare(release.Next) >= 0 {
			continue
		}

		if best == "" || parsed.Compare(bestVersion) > 0 {
			best = tag
			bestVersion = parsed
		}
	}

	return best, bestVersion
}

func (release *Release) parseTag(tag string) (semver.Version, bool) {
	prefix := release.Version.TagName(release.Folder, "")

	if release.Version.IsGit {
		pattern, compileError := version.ParseTag(release.Version.Patterns[0])
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
