package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	mkdirError := os.MkdirAll(filepath.Dir(path), 0o755)
	if mkdirError != nil {
		t.Fatal(mkdirError)
	}

	writeError := os.WriteFile(path, []byte(content), 0o644)
	if writeError != nil {
		t.Fatal(writeError)
	}

	return path
}

func readBack(t *testing.T, path string) string {
	t.Helper()

	data, readError := os.ReadFile(path)
	if readError != nil {
		t.Fatal(readError)
	}

	return string(data)
}

func TestAutoJavaScriptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := "{\n  \"name\": \"app\",\n  \"version\": \"1.4.0\",\n  \"scripts\": {\"build\": \"vite\"}\n}\n"
	path := writeFile(t, dir, "package.json", original)

	resolved, resolveError := Resolve(Settings{}, dir)
	if resolveError != nil {
		t.Fatal(resolveError)
	}

	if resolved.Preset != PRESET_JAVASCRIPT {
		t.Fatalf("preset = %s", resolved.Preset)
	}

	current, readError := Read(resolved, nil)
	if readError != nil {
		t.Fatal(readError)
	}

	if current.Version.String() != "1.4.0" {
		t.Fatalf("version = %s", current.Version)
	}

	files, writeError := current.Write(Fill{Version: "1.5.0"})
	if writeError != nil {
		t.Fatal(writeError)
	}

	if len(files) != 1 {
		t.Fatalf("files = %v", files)
	}

	want := strings.Replace(original, "1.4.0", "1.5.0", 1)
	if got := readBack(t, path); got != want {
		t.Fatalf("formatting changed:\n%s", got)
	}
}

func TestPythonPoetryFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[tool.poetry]\nname = \"app\"\nversion = \"0.3.1\"\n")

	resolved, resolveError := Resolve(Settings{Preset: PRESET_PYTHON}, dir)
	if resolveError != nil {
		t.Fatal(resolveError)
	}

	current, readError := Read(resolved, nil)
	if readError != nil {
		t.Fatal(readError)
	}

	if current.Version.String() != "0.3.1" {
		t.Fatalf("version = %s", current.Version)
	}
}

func TestPythonDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"app\"\ndynamic = [\"version\"]\n")

	resolved, resolveError := Resolve(Settings{}, dir)
	if resolveError != nil {
		t.Fatal(resolveError)
	}

	_, readError := Read(resolved, nil)
	if readError == nil || !strings.Contains(readError.Error(), "dynamic") {
		t.Fatalf("want a dynamic version error, got %v", readError)
	}
}

func TestGlobMustAgree(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "packages/a/package.json", `{"version": "2.0.0"}`)
	writeFile(t, dir, "packages/b/package.json", `{"version": "2.1.0"}`)

	resolved, resolveError := Resolve(Settings{Source: "packages/*/package.json"}, dir)
	if resolveError != nil {
		t.Fatal(resolveError)
	}

	_, readError := Read(resolved, nil)
	if readError == nil || !strings.Contains(readError.Error(), "differ") {
		t.Fatalf("want a mismatch error, got %v", readError)
	}
}

func TestCustomText(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "src/version.sh", "VERSION=\"0.1.0\"\nNAME=app\n")

	resolved, resolveError := Resolve(Settings{Preset: PRESET_CUSTOM, Source: "src/version.sh", Pattern: `VERSION="{version}"`}, dir)
	if resolveError != nil {
		t.Fatal(resolveError)
	}

	current, readError := Read(resolved, nil)
	if readError != nil {
		t.Fatal(readError)
	}

	_, writeError := current.Write(Fill{Version: "0.2.0"})
	if writeError != nil {
		t.Fatal(writeError)
	}

	if got := readBack(t, path); got != "VERSION=\"0.2.0\"\nNAME=app\n" {
		t.Fatalf("got %q", got)
	}
}

func TestGitTags(t *testing.T) {
	resolved, resolveError := Resolve(Settings{Preset: PRESET_GO, Pattern: "api/v{version}"}, t.TempDir())
	if resolveError != nil {
		t.Fatal(resolveError)
	}

	current, readError := Read(resolved, []string{"v9.0.0", "api/v1.2.0", "api/v1.10.0", "api/v1.9.9", "api/vX"})
	if readError != nil {
		t.Fatal(readError)
	}

	if current.Version.String() != "1.10.0" {
		t.Fatalf("version = %s", current.Version)
	}

	if tag := resolved.TagName("api", "1.11.0"); tag != "api/v1.11.0" {
		t.Fatalf("tag = %s", tag)
	}
}

func TestNoTagsStartsAtZero(t *testing.T) {
	resolved, _ := Resolve(Settings{Preset: PRESET_GIT}, t.TempDir())

	current, readError := Read(resolved, nil)
	if readError != nil || current.Version.String() != "0.0.0" {
		t.Fatalf("version = %v, %v", current, readError)
	}
}

func TestDetectErrors(t *testing.T) {
	_, noneError := Resolve(Settings{}, t.TempDir())
	if noneError == nil {
		t.Fatal("want an error with no stack")
	}

	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"version": "1.0.0"}`)
	writeFile(t, dir, "go.mod", "module x\n")

	_, severalError := Resolve(Settings{}, dir)
	if severalError == nil || !strings.Contains(severalError.Error(), "several") {
		t.Fatalf("want a several-stacks error, got %v", severalError)
	}
}

func TestTagNameFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"version": "1.0.0"}`)

	resolved, _ := Resolve(Settings{}, dir)

	if resolved.TagName(".", "1.0.1") != "v1.0.1" || resolved.TagName("web", "1.0.1") != "web/v1.0.1" {
		t.Fatal("TagName")
	}
}

func TestLocks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"version": "1.0.0"}`)
	writeFile(t, dir, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")

	resolved, _ := Resolve(Settings{}, dir)

	locks, lockError := resolved.Locks()
	if lockError != nil || len(locks) != 1 || locks[0].Program != "pnpm" {
		t.Fatalf("locks = %+v, %v", locks, lockError)
	}

	writeFile(t, dir, "package-lock.json", "{}")

	_, twoError := resolved.Locks()
	if twoError == nil {
		t.Fatal("want an error with two lock files")
	}
}
