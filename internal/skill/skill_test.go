package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall(t *testing.T) {
	root := t.TempDir()
	tool, _ := LookupTool("claude")

	path, pathError := Path(tool, SCOPE_PROJECT, root, "/home/none")
	if pathError != nil {
		t.Fatal(pathError)
	}

	if path != filepath.Join(root, ".claude", "skills", "shipo", "SKILL.md") {
		t.Fatalf("path = %s", path)
	}

	previous, installError := Install(path, "1.0.0")
	if installError != nil || previous != "" {
		t.Fatalf("first install: %q %v", previous, installError)
	}

	previous, installError = Install(path, "1.1.0")
	if installError != nil || previous != "1.0.0" {
		t.Fatalf("second install: %q %v", previous, installError)
	}

	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "---\nname: shipo\n") || !strings.Contains(string(data), "shipo-version: 1.1.0\n---\n") {
		t.Fatalf("content:\n%s", data)
	}
}

func TestTools(t *testing.T) {
	names := ToolNames()
	if strings.Join(names, ",") != "agents,claude,codex,cursor,copilot" {
		t.Fatalf("tools = %v", names)
	}

	for _, name := range names {
		tool, isFound := LookupTool(name)
		if !isFound || tool.Project == "" || tool.User == "" {
			t.Fatalf("tool %s: %+v", name, tool)
		}
	}

	_, isFound := LookupTool("vim")
	if isFound {
		t.Fatal("unknown tool")
	}
}

func TestPaths(t *testing.T) {
	cases := map[string][2]string{
		"agents":  {filepath.Join("/repo", ".agents", "skills", "shipo", "SKILL.md"), filepath.Join("/home", ".agents", "skills", "shipo", "SKILL.md")},
		"codex":   {filepath.Join("/repo", ".agents", "skills", "shipo", "SKILL.md"), filepath.Join("/home", ".codex", "skills", "shipo", "SKILL.md")},
		"copilot": {filepath.Join("/repo", ".github", "skills", "shipo", "SKILL.md"), filepath.Join("/home", ".copilot", "skills", "shipo", "SKILL.md")},
	}

	for name, want := range cases {
		tool, _ := LookupTool(name)

		project, _ := Path(tool, SCOPE_PROJECT, "/repo", "/home")
		user, _ := Path(tool, SCOPE_USER, "/repo", "/home")

		if project != want[0] || user != want[1] {
			t.Fatalf("%s: %s, %s", name, project, user)
		}
	}

	tool, _ := LookupTool("claude")

	_, scopeError := Path(tool, "global", "/repo", "/home")
	if scopeError == nil {
		t.Fatal("unknown scope")
	}
}

func TestInstallErrors(t *testing.T) {
	dir := t.TempDir()

	_, readError := Install(dir, "1.0.0")
	if readError == nil {
		t.Fatal("installing over a folder must fail")
	}

	foreign := filepath.Join(dir, "SKILL.md")
	_ = os.WriteFile(foreign, []byte("hand written"), 0o644)

	previous, installError := Install(foreign, "1.0.0")
	if installError != nil || previous != "unknown" {
		t.Fatalf("previous = %q, %v", previous, installError)
	}
}

func TestContentIsValid(t *testing.T) {
	if !strings.HasPrefix(Content, "---\nname: shipo\ndescription: ") || !strings.Contains(Content, "\n---\n") {
		t.Fatal("SKILL.md must start with name and description front matter")
	}

	for _, needed := range []string{"[version]", "[bump]", "[build]", "[publish]", "shipo check", "Never run `shipo`"} {
		if !strings.Contains(Content, needed) {
			t.Fatalf("SKILL.md misses %q", needed)
		}
	}
}
