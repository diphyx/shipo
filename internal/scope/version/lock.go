package version

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/diphyx/shipo/internal/tool/process"
)

// lockTool is a lock file and the command that refreshes it.
type lockTool struct {
	file    string
	program string
	command []string
}

var lockTools = map[string][]lockTool{
	PRESET_JAVASCRIPT: {
		{file: "package-lock.json", program: "npm", command: []string{"npm", "install", "--package-lock-only", "--ignore-scripts"}},
		{file: "yarn.lock", program: "yarn", command: []string{"yarn", "install", "--mode=update-lockfile"}},
		{file: "pnpm-lock.yaml", program: "pnpm", command: []string{"pnpm", "install", "--lockfile-only", "--ignore-scripts"}},
		{file: "bun.lock", program: "bun", command: []string{"bun", "install", "--lockfile-only"}},
		{file: "bun.lockb", program: "bun", command: []string{"bun", "install", "--lockfile-only"}},
	},
	PRESET_RUST: {
		{file: "Cargo.lock", program: "cargo", command: []string{"cargo", "update", "--workspace", "--offline"}},
	},
	PRESET_PYTHON: {
		{file: "uv.lock", program: "uv", command: []string{"uv", "lock"}},
		{file: "poetry.lock", program: "poetry", command: []string{"poetry", "lock"}},
		{file: "pdm.lock", program: "pdm", command: []string{"pdm", "lock", "--update-reuse"}},
	},
}

// Lock is a lock file to refresh after the version is written.
type Lock struct {
	File    string
	Program string
	Command process.Command
}

// Locks finds the lock files next to the version files, one per folder.
// Two lock files of the same stack in one folder is an error.
func (resolved *Resolved) Locks() ([]Lock, error) {
	tools := lockTools[resolved.Preset]
	if len(tools) == 0 {
		return nil, nil
	}

	seen := map[string]bool{}

	var locks []Lock

	for _, file := range resolved.Files {
		dir := filepath.Dir(file)
		if seen[dir] {
			continue
		}

		seen[dir] = true

		var found []lockTool

		for _, tool := range tools {
			_, statError := os.Stat(filepath.Join(dir, tool.file))
			if statError == nil {
				found = append(found, tool)
			}
		}

		if len(found) > 1 {
			names := make([]string, len(found))
			for index, tool := range found {
				names[index] = tool.file
			}

			return nil, fmt.Errorf("%s has several lock files (%s); keep one", relative(resolved.Dir, dir), strings.Join(names, ", "))
		}

		if len(found) == 1 {
			tool := found[0]
			locks = append(locks, Lock{
				File:    filepath.Join(dir, tool.file),
				Program: tool.program,
				Command: process.Command{Name: tool.command[0], Arguments: tool.command[1:], Dir: dir},
			})
		}
	}

	return locks, nil
}

// TagName returns the tag for a version. Git sources use their pattern; file
// sources use v{version}, prefixed with the target folder when it isn't the
// repository root.
func (resolved *Resolved) TagName(folder string, version string) string {
	if resolved.IsGit {
		return Render(resolved.Patterns[0], Fill{Version: version})
	}

	if folder == "" || folder == "." {
		return "v" + version
	}

	return filepath.ToSlash(folder) + "/v" + version
}
