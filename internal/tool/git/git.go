// Package git wraps the git CLI. shipo never reimplements git, so hooks,
// signing, SSH keys and credentials all apply.
package git

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/diphyx/shipo/internal/tool/process"
)

// PROGRAM is the git executable.
const PROGRAM = "git"

// Git runs git commands in one folder.
type Git struct {
	Runner process.Runner
	Dir    string
}

// NewGit returns a Git that runs in dir.
func NewGit(runner process.Runner, dir string) *Git {
	return &Git{Runner: runner, Dir: dir}
}

func (git *Git) command(arguments ...string) process.Command {
	return process.Command{Name: PROGRAM, Arguments: arguments, Dir: git.Dir}
}

func (git *Git) output(context context.Context, arguments ...string) (string, error) {
	return git.Runner.Output(context, git.command(arguments...))
}

func (git *Git) run(context context.Context, arguments ...string) error {
	_, runError := git.output(context, arguments...)

	return runError
}

// Root returns the top folder of the repository.
func (git *Git) Root(context context.Context) (string, error) {
	root, rootError := git.output(context, "rev-parse", "--show-toplevel")
	if rootError != nil {
		return "", errors.New("not a git repository")
	}

	return filepath.FromSlash(root), nil
}

// IsClean reports whether the working tree has no changes, tracked or untracked.
func (git *Git) IsClean(context context.Context) (bool, error) {
	status, statusError := git.output(context, "status", "--porcelain")

	return status == "", statusError
}

// ChangedTracked lists tracked files that differ from HEAD.
func (git *Git) ChangedTracked(context context.Context) ([]string, error) {
	changed, diffError := git.output(context, "diff", "--name-only", "HEAD")
	if diffError != nil || changed == "" {
		return nil, diffError
	}

	return strings.Split(changed, "\n"), nil
}

// Head returns the full hash of HEAD.
func (git *Git) Head(context context.Context) (string, error) {
	return git.output(context, "rev-parse", "HEAD")
}

// ShortHash returns the short hash of a revision.
func (git *Git) ShortHash(context context.Context, revision string) (string, error) {
	return git.output(context, "rev-parse", "--short", revision)
}

// Branch returns the current branch, or "" when HEAD is detached.
func (git *Git) Branch(context context.Context) (string, error) {
	branch, branchError := git.output(context, "branch", "--show-current")

	return branch, branchError
}

// Subject returns the first line of a commit message.
func (git *Git) Subject(context context.Context, revision string) (string, error) {
	return git.output(context, "log", "-1", "--format=%s", revision)
}

// Tags lists every local tag.
func (git *Git) Tags(context context.Context) ([]string, error) {
	tags, tagsError := git.output(context, "tag", "--list")
	if tagsError != nil || tags == "" {
		return nil, tagsError
	}

	return strings.Split(tags, "\n"), nil
}

// TagsAt lists the tags that point at a revision.
func (git *Git) TagsAt(context context.Context, revision string) ([]string, error) {
	tags, tagsError := git.output(context, "tag", "--points-at", revision)
	if tagsError != nil || tags == "" {
		return nil, tagsError
	}

	return strings.Split(tags, "\n"), nil
}

// HasTag reports whether a local tag exists.
func (git *Git) HasTag(context context.Context, tag string) bool {
	return git.run(context, "rev-parse", "--verify", "--quiet", "refs/tags/"+tag) == nil
}

// HasRemoteTag reports whether a tag exists on a remote.
func (git *Git) HasRemoteTag(context context.Context, remote string, tag string) (bool, error) {
	output, listError := git.output(context, "ls-remote", "--tags", remote, "refs/tags/"+tag)

	return output != "", listError
}

// HasRemote reports whether a remote is configured.
func (git *Git) HasRemote(context context.Context, remote string) bool {
	return git.run(context, "remote", "get-url", remote) == nil
}

// RemoteURL returns the fetch URL of a remote.
func (git *Git) RemoteURL(context context.Context, remote string) (string, error) {
	return git.output(context, "remote", "get-url", remote)
}

// Fetch fetches a remote's branches. Tags are left alone, so a local tag that
// differs from the remote's, as after current moved it, can't fail the fetch.
func (git *Git) Fetch(context context.Context, remote string) error {
	return git.run(context, "fetch", "--quiet", "--no-tags", remote)
}

// Resolve returns the hash of a revision, or "" if it doesn't exist.
func (git *Git) Resolve(context context.Context, revision string) string {
	hash, resolveError := git.output(context, "rev-parse", "--verify", "--quiet", revision+"^{commit}")
	if resolveError != nil {
		return ""
	}

	return hash
}

// IsAncestor reports whether ancestor is reachable from descendant.
func (git *Git) IsAncestor(context context.Context, ancestor string, descendant string) bool {
	return git.run(context, "merge-base", "--is-ancestor", ancestor, descendant) == nil
}

// Add stages files.
func (git *Git) Add(context context.Context, paths ...string) error {
	arguments := append([]string{"add", "--"}, paths...)

	return git.run(context, arguments...)
}

// Commit commits the staged changes. An empty commit is allowed.
func (git *Git) Commit(context context.Context, message string, isEmptyAllowed bool) error {
	arguments := []string{"commit", "--quiet", "--message", message}
	if isEmptyAllowed {
		arguments = append(arguments, "--allow-empty")
	}

	return git.run(context, arguments...)
}

// Tag creates an annotated tag at HEAD.
func (git *Git) Tag(context context.Context, tag string, message string) error {
	return git.TagAt(context, tag, "HEAD", message)
}

// TagAt creates an annotated tag at a revision.
func (git *Git) TagAt(context context.Context, tag string, revision string, message string) error {
	return git.run(context, "tag", "--annotate", tag, revision, "--message", message)
}

// DeleteTag removes a local tag.
func (git *Git) DeleteTag(context context.Context, tag string) error {
	return git.run(context, "tag", "--delete", tag)
}

// ResetHard moves the branch and the working tree to a revision.
func (git *Git) ResetHard(context context.Context, revision string) error {
	return git.run(context, "reset", "--hard", "--quiet", revision)
}

// MoveBranch points a branch that isn't checked out at a revision.
func (git *Git) MoveBranch(context context.Context, branch string, revision string) error {
	return git.run(context, "branch", "--force", branch, revision)
}

// DeleteRemoteTag deletes a tag on a remote. Its output goes to output, or to
// the runner's streams when output is nil.
func (git *Git) DeleteRemoteTag(context context.Context, output io.Writer, remote string, tag string) error {
	command := git.command("push", remote, "--delete", "refs/tags/"+tag)
	command.Output = output

	return git.Runner.Run(context, command)
}

// PushAtomic pushes refspecs to a remote in one atomic push. Its output goes
// to output, or to the runner's streams when output is nil.
func (git *Git) PushAtomic(context context.Context, output io.Writer, remote string, refspecs ...string) error {
	command := git.command(append([]string{"push", "--atomic", remote}, refspecs...)...)
	command.Output = output

	return git.Runner.Run(context, command)
}
