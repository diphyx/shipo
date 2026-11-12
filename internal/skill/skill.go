// Package skill embeds the AI skill and installs it for an AI tool.
package skill

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "embed"
)

// Content is the SKILL.md installed by `shipo skill`.
//
//go:embed SKILL.md
var Content string

// The install scopes.
const (
	SCOPE_PROJECT = "project"
	SCOPE_USER    = "user"
)

// Tool is an AI tool and where it reads skills.
type Tool struct {
	Name    string
	Project string
	User    string
}

// Tools lists the supported AI tools. The paths are relative to the project
// root or to the home folder.
var Tools = []Tool{
	{Name: "agents", Project: ".agents/skills", User: ".agents/skills"},
	{Name: "claude", Project: ".claude/skills", User: ".claude/skills"},
	{Name: "codex", Project: ".agents/skills", User: ".codex/skills"},
	{Name: "cursor", Project: ".cursor/skills", User: ".cursor/skills"},
	{Name: "copilot", Project: ".github/skills", User: ".copilot/skills"},
}

// LookupTool returns a tool by name.
func LookupTool(name string) (Tool, bool) {
	for _, tool := range Tools {
		if tool.Name == name {
			return tool, true
		}
	}

	return Tool{}, false
}

// ToolNames lists the tool names.
func ToolNames() []string {
	names := make([]string, len(Tools))
	for index, tool := range Tools {
		names[index] = tool.Name
	}

	return names
}

// Path returns where the skill file goes for a tool and scope.
func Path(tool Tool, scope string, projectRoot string, home string) (string, error) {
	switch scope {
	case SCOPE_PROJECT:
		{
			return filepath.Join(projectRoot, filepath.FromSlash(tool.Project), "shipo", "SKILL.md"), nil
		}
	case SCOPE_USER:
		{
			return filepath.Join(home, filepath.FromSlash(tool.User), "shipo", "SKILL.md"), nil
		}
	}

	return "", fmt.Errorf("unknown scope %q: want project or user", scope)
}

var versionLine = regexp.MustCompile(`(?m)^shipo-version: (.+)$`)

// Render returns the skill content stamped with the shipo version.
func Render(version string) string {
	return strings.Replace(Content, "\n---\n", "\nshipo-version: "+version+"\n---\n", 1)
}

// Install writes the skill. It returns the version it replaced, if any.
func Install(path string, version string) (string, error) {
	previous := ""

	data, readError := os.ReadFile(path)
	if readError == nil {
		match := versionLine.FindSubmatch(data)
		if match != nil {
			previous = string(match[1])
		} else {
			previous = "unknown"
		}
	} else if !errors.Is(readError, os.ErrNotExist) {
		return "", readError
	}

	mkdirError := os.MkdirAll(filepath.Dir(path), 0o755)
	if mkdirError != nil {
		return "", mkdirError
	}

	writeError := os.WriteFile(path, []byte(Render(version)), 0o644)
	if writeError != nil {
		return "", writeError
	}

	return previous, nil
}
