package release

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/tool/git"
	"github.com/diphyx/shipo/internal/tool/process"
)

// ROOT_FOLDER is the repository's top folder, as a target.
const ROOT_FOLDER = "."

// Targets is every .shipo in a repository.
type Targets struct {
	// Root is the repository's top folder.
	Root string
	// Folders hold a .shipo each, relative to Root and slash-separated: "."
	// first when Root has one, then the others by path.
	Folders []string
}

// NewTargets finds every .shipo in the repository that holds dir, skipping what
// git ignores.
func NewTargets(context context.Context, runner process.Runner, dir string) (*Targets, error) {
	repository := git.NewGit(runner, dir)

	root, rootError := repository.Root(context)
	if rootError != nil {
		return nil, rootError
	}

	files, filesError := repository.Files(context, ":(top,glob)**/"+config.FILE_NAME)
	if filesError != nil {
		return nil, filesError
	}

	targets := &Targets{Root: root}
	seen := map[string]bool{}

	for _, file := range files {
		folder := path.Dir(file)
		if seen[folder] || path.Base(file) != config.FILE_NAME || !isFile(filepath.Join(root, filepath.FromSlash(file))) {
			continue
		}

		seen[folder] = true
		targets.Folders = append(targets.Folders, folder)
	}

	sort.Slice(targets.Folders, func(left int, right int) bool {
		first, second := targets.Folders[left], targets.Folders[right]
		if first == ROOT_FOLDER || second == ROOT_FOLDER {
			return first == ROOT_FOLDER
		}

		return first < second
	})

	return targets, nil
}

// Dir is the absolute path of a target folder.
func (targets *Targets) Dir(folder string) string {
	return filepath.Join(targets.Root, filepath.FromSlash(folder))
}

// Match finds a target by its folder, or by its last folder name when only
// one target has it.
func (targets *Targets) Match(name string) (string, error) {
	if len(targets.Folders) == 0 {
		return "", config.ErrNotFound
	}

	wanted := path.Clean(filepath.ToSlash(name))

	var matches []string

	for _, folder := range targets.Folders {
		if folder == wanted {
			return folder, nil
		}

		if folder != ROOT_FOLDER && path.Base(folder) == wanted {
			matches = append(matches, folder)
		}
	}

	switch len(matches) {
	case 0:
		{
			return "", fmt.Errorf("no target %q; targets: %s", name, targets)
		}
	case 1:
		{
			return matches[0], nil
		}
	}

	return "", fmt.Errorf("target %q is ambiguous: %s; give its path", name, strings.Join(matches, ", "))
}

// Nearest is the folder of the nearest .shipo from dir upwards, or "" when it
// is not one of the targets.
func (targets *Targets) Nearest(dir string) string {
	found, findError := config.Find(dir)
	if findError != nil {
		return ""
	}

	resolvedRoot, _ := filepath.EvalSymlinks(targets.Root)
	resolvedDir, _ := filepath.EvalSymlinks(filepath.Dir(found))

	relative, relativeError := filepath.Rel(resolvedRoot, resolvedDir)
	if relativeError != nil {
		return ""
	}

	folder := filepath.ToSlash(relative)

	for _, target := range targets.Folders {
		if target == folder {
			return folder
		}
	}

	return ""
}

// String lists the folders, comma-separated.
func (targets *Targets) String() string {
	return strings.Join(targets.Folders, ", ")
}

func isFile(name string) bool {
	info, statError := os.Stat(name)

	return statError == nil && !info.IsDir()
}
