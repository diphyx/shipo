package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/ui"
)

func TestShowVersion(t *testing.T) {
	current := newSession(t, "", nil)

	if current.run(t, "", false, "version") != nil {
		t.Fatal("version failed")
	}

	want := "target   .\npreset   javascript\nsource   package.json\nversion  1.0.0\ntag      none yet\n"
	if current.output.String() != want {
		t.Fatalf("got:\n%s", current.output)
	}

	current.repo.Git("tag", "v1.0.0")
	current.output.Reset()
	_ = current.run(t, "", false, "version")

	if !strings.HasSuffix(current.output.String(), "tag      v1.0.0\n") {
		t.Fatalf("got:\n%s", current.output)
	}
}

func TestShowVersionGitAndSubfolder(t *testing.T) {
	current := newSession(t, "[version]\npreset = \"go\"\n", map[string]string{"go.mod": "module x\n"})
	current.repo.Write(map[string]string{"api/.shipo": "[version]\npreset = \"git\"\npattern = \"api/v{version}\"\n"})

	app := current.app("", false)
	app.Dir = filepath.Join(current.repo.Work, "api")

	if app.Dispatch(context.Background(), []string{"version"}) != nil {
		t.Fatal("version failed")
	}

	if !strings.Contains(current.output.String(), "target   api/\n") || !strings.Contains(current.output.String(), "source   git tags (api/v{version})\n") {
		t.Fatalf("got:\n%s", current.output)
	}
}

func TestCheckCommand(t *testing.T) {
	current := newSession(t, "[publish]\ncommand = \"npm publish\"\n", nil)

	checkError := current.run(t, "", false, "check")
	if checkError != nil {
		t.Fatalf("check: %v\n%s", checkError, current.output)
	}

	text := current.output.String()
	for _, line := range []string{"checks\n", "✓ tree      clean", "HEAD is not a release", "plan\n", "push      origin/main + v1.0.1", "command   npm publish"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}
}

func TestCheckCommandAtRelease(t *testing.T) {
	current := newSession(t, "", nil)
	current.repo.Git("tag", "v1.0.0")

	_ = current.run(t, "", false, "check")

	text := current.output.String()
	if !strings.Contains(text, "release v1.0.0\n") || !strings.Contains(text, "✗ tag       local, not pushed") || !strings.Contains(text, "push      origin/main + v1.0.0") {
		t.Fatalf("got:\n%s", text)
	}

	if strings.Contains(text, "tree") {
		t.Fatal("at a release, the tree is not checked for a bump")
	}
}

func TestCheckCommandFails(t *testing.T) {
	current := newSession(t, "", nil)
	current.repo.Write(map[string]string{"dirty.txt": "x"})

	checkError := current.run(t, "", false, "check")
	if checkError == nil || !strings.Contains(checkError.Error(), "tree") {
		t.Fatalf("want a failed check, got %v", checkError)
	}

	if !strings.Contains(current.output.String(), "✗ tree") {
		t.Fatal("the failure must also be printed")
	}
}

func TestInit(t *testing.T) {
	testkit.Isolate(t)

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nversion = \"0.1.0\"\n"), 0o644)

	var output bytes.Buffer

	app := &App{UI: ui.NewTestTerminal(&output, "", false), Runner: testkit.NewRunner(), Dir: dir}

	if app.Dispatch(context.Background(), []string{"init"}) != nil {
		t.Fatal("init failed")
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".shipo"))
	if !strings.Contains(string(data), "preset = \"rust\"") || !strings.Contains(output.String(), "✓ init       .shipo (rust)") {
		t.Fatalf(".shipo:\n%s\noutput:\n%s", data, output.String())
	}

	if app.Dispatch(context.Background(), []string{"init"}) == nil {
		t.Fatal("init must not overwrite a .shipo")
	}

	app.Dir = t.TempDir()
	if app.Dispatch(context.Background(), []string{"init"}) == nil {
		t.Fatal("init needs a stack")
	}
}

func TestInitOutputLoads(t *testing.T) {
	current := newSession(t, "", nil)
	_ = os.Remove(filepath.Join(current.repo.Work, ".shipo"))

	if current.run(t, "", false, "init") != nil {
		t.Fatal("init failed")
	}

	_, openError := release.Open(context.Background(), current.runner, current.repo.Work)
	if openError != nil {
		t.Fatalf("the generated .shipo must load: %v", openError)
	}
}

func TestSkill(t *testing.T) {
	current := newSession(t, "", nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if current.run(t, "", false, "skill", "claude", "project") != nil {
		t.Fatal("skill project failed")
	}

	projectFile := filepath.Join(current.repo.Work, ".claude", "skills", "shipo", "SKILL.md")

	data, readError := os.ReadFile(projectFile)
	if readError != nil || !strings.Contains(string(data), "shipo-version: 9.9.9") {
		t.Fatalf("project skill: %v\n%s", readError, data)
	}

	if current.run(t, "", false, "skill", "copilot", "user") != nil {
		t.Fatal("skill user failed")
	}

	_, statError := os.Stat(filepath.Join(home, ".copilot", "skills", "shipo", "SKILL.md"))
	if statError != nil {
		t.Fatal("user skill not installed")
	}

	current.output.Reset()

	if current.run(t, "", false, "skill", "claude", "project") != nil || !strings.Contains(current.output.String(), "(replaced 9.9.9 with 9.9.9)") {
		t.Fatalf("reinstall output: %s", current.output)
	}
}

func TestSkillMenusAndErrors(t *testing.T) {
	current := newSession(t, "", nil)
	t.Setenv("HOME", t.TempDir())

	if current.run(t, "2\n1\n", true, "skill") != nil {
		t.Fatalf("menu install failed:\n%s", current.output)
	}

	_, statError := os.Stat(filepath.Join(current.repo.Work, ".claude", "skills", "shipo", "SKILL.md"))
	if statError != nil {
		t.Fatal("the menu picks (claude, project) did not install")
	}

	noTerminal := current.run(t, "", false, "skill")
	if !errors.Is(noTerminal, ui.ErrNoTerminal) || !strings.Contains(noTerminal.Error(), "usage: shipo skill") {
		t.Fatalf("without a terminal, want usage, got %v", noTerminal)
	}

	scopeMenu := current.run(t, "", false, "skill", "cursor")
	if !errors.Is(scopeMenu, ui.ErrNoTerminal) {
		t.Fatalf("want ErrNoTerminal for the scope menu, got %v", scopeMenu)
	}

	if current.run(t, "", false, "skill", "vim", "project") == nil {
		t.Fatal("unknown tool")
	}

	if current.run(t, "", false, "skill", "claude", "everywhere") == nil {
		t.Fatal("unknown scope")
	}
}
