package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	config, parseError := Parse("")
	if parseError != nil {
		t.Fatal(parseError)
	}

	if config.Version.Preset != "auto" || config.Bump.Level != "ask" || config.Bump.Commit != DEFAULT_COMMIT {
		t.Fatalf("defaults not applied: %+v", config)
	}

	if !reflect.DeepEqual(config.Publish.Push.Items, []string{"origin/main"}) {
		t.Fatalf("push default = %+v", config.Publish.Push)
	}

	if config.Publish.Release.Enabled() || config.Publish.Workflow.Enabled() {
		t.Fatal("release and workflow should be off by default")
	}
}

func TestParseFull(t *testing.T) {
	content := `
[version]
preset  = "custom"
source  = "src/version.sh"
pattern = 'VERSION="{version}"'

[bump]
level  = "patch"
commit = "release: {tag}"

[build]
dir         = "web"
command     = ["npm ci", "npm run build"]
environment = "APP_VERSION={version}"

[publish]
push     = ["origin/main", "mirror/main"]
release  = true
workflow = "build.yml version={tag}"
command  = "npm publish"
`

	config, parseError := Parse(content)
	if parseError != nil {
		t.Fatal(parseError)
	}

	if !reflect.DeepEqual([]string(config.Build.Command), []string{"npm ci", "npm run build"}) {
		t.Fatalf("build.command = %q", config.Build.Command)
	}

	if !reflect.DeepEqual([]string(config.Build.Environment), []string{"APP_VERSION={version}"}) {
		t.Fatalf("build.environment = %q", config.Build.Environment)
	}

	if len(config.Publish.Push.Items) != 2 || !config.Publish.Release.IsOn || !config.Publish.Release.Enabled() {
		t.Fatalf("publish = %+v", config.Publish)
	}

	if config.Publish.Workflow.Items[0] != "build.yml version={tag}" {
		t.Fatalf("workflow = %+v", config.Publish.Workflow)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"unknown key":      "[bump]\nnope = 1",
		"push true":        "[publish]\npush = true",
		"push no branch":   "[publish]\npush = \"origin\"",
		"workflow true":    "[publish]\nworkflow = true",
		"bad environment":  "[build]\ncommand = \"x\"\nenvironment = \"NOEQUALS\"",
		"empty list":       "[build]\ncommand = []",
		"non-string entry": "[publish]\ncommand = [1]",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, parseError := Parse(content)
			if parseError == nil {
				t.Fatalf("want an error for %q", content)
			}
		})
	}
}

func TestPushOff(t *testing.T) {
	config, parseError := Parse("[publish]\npush = false")
	if parseError != nil {
		t.Fatal(parseError)
	}

	if config.Publish.Push.Enabled() {
		t.Fatal("push = false should be disabled")
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")

	mkdirError := os.MkdirAll(nested, 0o755)
	if mkdirError != nil {
		t.Fatal(mkdirError)
	}

	writeError := os.WriteFile(filepath.Join(root, FILE_NAME), nil, 0o644)
	if writeError != nil {
		t.Fatal(writeError)
	}

	found, findError := Find(nested)
	if findError != nil {
		t.Fatal(findError)
	}

	resolvedRoot, _ := filepath.EvalSymlinks(root)
	resolvedFound, _ := filepath.EvalSymlinks(filepath.Dir(found))

	if resolvedFound != resolvedRoot {
		t.Fatalf("Find = %s, want %s", found, root)
	}
}

func TestSplitPush(t *testing.T) {
	remote, branch, splitError := SplitPush("origin/release/v2")
	if splitError != nil || remote != "origin" || branch != "release/v2" {
		t.Fatalf("got %q %q %v", remote, branch, splitError)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FILE_NAME)

	_, missingError := Load(path)
	if missingError == nil {
		t.Fatal("want an error for a missing file")
	}

	_ = os.WriteFile(path, []byte("[bump]\nlevel = \"patch\"\n"), 0o644)

	config, loadError := Load(path)
	if loadError != nil || config.Bump.Level != "patch" {
		t.Fatalf("Load: %+v %v", config, loadError)
	}

	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedConfig, _ := filepath.EvalSymlinks(config.Dir)

	if resolvedConfig != resolvedDir || filepath.Base(config.Path) != FILE_NAME {
		t.Fatalf("Path %s, Dir %s", config.Path, config.Dir)
	}

	if config.Rel(filepath.Join(config.Dir, "web", "package.json")) != filepath.Join("web", "package.json") {
		t.Fatal("Rel")
	}

	_ = os.WriteFile(path, []byte("[bump]\nlevel = true\n"), 0o644)

	_, badError := Load(path)
	if badError == nil || !strings.Contains(badError.Error(), path) {
		t.Fatalf("the error must name the file, got %v", badError)
	}
}

func TestFindNotFound(t *testing.T) {
	_, findError := Find(t.TempDir())
	if !errors.Is(findError, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", findError)
	}
}

func TestFindIgnoresFolder(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, FILE_NAME), 0o755)

	_, findError := Find(root)
	if !errors.Is(findError, ErrNotFound) {
		t.Fatalf("a folder named .shipo is not a config, got %v", findError)
	}
}

func TestSplitPushErrors(t *testing.T) {
	for _, target := range []string{"origin", "/main", "origin/", ""} {
		_, _, splitError := SplitPush(target)
		if splitError == nil {
			t.Fatalf("SplitPush(%q) should fail", target)
		}
	}
}

func TestErrorKinds(t *testing.T) {
	cases := map[string]string{
		"[bump]\nlevel = [\"a\"]\n":   "got a list",
		"[build]\ncommand = true\n":   "got a boolean",
		"[publish]\npush = [true]\n":  "want a quoted string",
		"publish:\n  push: [true]\n":  "is not remote/branch",
		"bump:\n  level:\n    x: y\n": "got a section",
		"bump = \"x\"\n":              "want a section",
	}

	for content, want := range cases {
		_, parseError := Parse(content)
		if parseError == nil || !strings.Contains(parseError.Error(), want) {
			t.Fatalf("%q: want %q, got %v", content, want, parseError)
		}
	}
}

func TestCommentsOnly(t *testing.T) {
	config, parseError := Parse("# nothing set\n\n")
	if parseError != nil {
		t.Fatal(parseError)
	}

	if config.Bump.Level != DEFAULT_LEVEL {
		t.Fatal("defaults not applied")
	}
}
