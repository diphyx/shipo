package cli

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// newMonorepo is a repository with packages/api, packages/web and a deep
// tools/a/b/cli target. isRoot keeps the root's own .shipo.
func newMonorepo(t *testing.T, isRoot bool) *session {
	t.Helper()

	current := newSession(t, "", map[string]string{
		"package.json":                  PACKAGE_JSON,
		"packages/api/package.json":     "{\"version\": \"2.0.0\"}\n",
		"packages/api/.shipo":           "",
		"packages/web/package.json":     "{\"version\": \"3.0.0\"}\n",
		"packages/web/.shipo":           "",
		"tools/a/b/cli/package.json":    "{\"version\": \"4.0.0\"}\n",
		"tools/a/b/cli/.shipo":          "",
		"node_modules/dep/.shipo":       "",
		"node_modules/dep/package.json": "{\"version\": \"9.0.0\"}\n",
		".gitignore":                    "node_modules/\n",
	})

	if !isRoot {
		current.repo.Git("rm", "--quiet", ".shipo")
		current.repo.Commit("no root target")
	}

	return current
}

func TestTakeTarget(t *testing.T) {
	cases := []struct {
		arguments []string
		rest      []string
		target    string
	}{
		{[]string{"-t", "api", "patch"}, []string{"patch"}, "api"},
		{[]string{"bump", "patch", "--target", "web"}, []string{"bump", "patch"}, "web"},
		{[]string{"--target=packages/api", "build"}, []string{"build"}, "packages/api"},
		{[]string{"check"}, []string{"check"}, ""},
	}

	for _, item := range cases {
		rest, target, takeError := takeTarget(item.arguments)
		if takeError != nil || target != item.target || !reflect.DeepEqual(rest, item.rest) {
			t.Fatalf("%v: got %v %q %v", item.arguments, rest, target, takeError)
		}
	}

	for _, arguments := range [][]string{{"-t"}, {"build", "--target"}, {"-t", "--help"}, {"--target="}} {
		_, _, takeError := takeTarget(arguments)
		if takeError == nil {
			t.Fatalf("%v: want an error", arguments)
		}
	}
}

func TestTargetLabels(t *testing.T) {
	folders := []string{".", "api", "packages/api", "a/b/c", "a/b/c/d", "x/one/two/end", "y/one/two/end", "z/q/r/s/end"}
	want := []string{".", "api", "packages/api", "a/b/c", "a/../d", "x/../end", "y/../end", "z/../end"}

	if labels := targetLabels(folders); !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %v", labels)
	}

	same := []string{"x/one/two/end", "x/three/four/end"}
	if labels := targetLabels(same); !reflect.DeepEqual(labels, same) {
		t.Fatalf("labels that would be the same keep their path: %v", labels)
	}
}

func TestTargetMenuListsEveryShipo(t *testing.T) {
	current := newMonorepo(t, true)

	versionError := current.run(t, "4\n", true, "version")
	if versionError != nil {
		t.Fatalf("version: %v\n%s", versionError, current.output)
	}

	text := current.output.String()

	for _, line := range []string{"1) .            → 1.0.0", "2) packages/api → 2.0.0", "3) packages/web → 3.0.0", "4) tools/../cli → 4.0.0"} {
		if !strings.Contains(text, line) {
			t.Fatalf("menu misses %q:\n%s", line, text)
		}
	}

	if strings.Contains(text, "node_modules") {
		t.Fatalf("ignored folders are not targets:\n%s", text)
	}

	if !strings.Contains(text, "target   tools/a/b/cli/") || strings.Contains(text, "✓ target") || !strings.Contains(text, "4.0.0") {
		t.Fatalf("the chosen target is used:\n%s", text)
	}
}

func TestTargetMenuSkipsMissingRoot(t *testing.T) {
	current := newMonorepo(t, false)

	flowError := current.run(t, "2\n1\n", true)
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	if strings.Contains(current.output.String(), "1) .") {
		t.Fatalf("the root has no .shipo, so it is not listed:\n%s", current.output)
	}

	if !strings.Contains(current.output.String(), "✓ target     packages/web") {
		t.Fatalf("the flow shows the chosen target:\n%s", current.output)
	}

	if tags := current.repo.RemoteGit("tag", "--list"); tags != "packages/web/v3.0.1" {
		t.Fatalf("remote tags = %q\n%s", tags, current.output)
	}
}

func TestTargetFlag(t *testing.T) {
	current := newMonorepo(t, false)

	bumpError := current.run(t, "", false, "-t", "api", "bump", "minor")
	if bumpError != nil {
		t.Fatalf("bump: %v\n%s", bumpError, current.output)
	}

	if tags := current.repo.Git("tag", "--list"); tags != "packages/api/v2.1.0" {
		t.Fatalf("tags = %q", tags)
	}

	if strings.Contains(current.output.String(), "target ") {
		t.Fatalf("steps print no target line:\n%s", current.output)
	}

	if current.run(t, "", false, "-t", "nope", "build") == nil {
		t.Fatal("an unknown target must fail")
	}

	if current.run(t, "", false, "-t", "api", "init") == nil {
		t.Fatal("init takes no -t")
	}
}

func TestStepsWithoutTarget(t *testing.T) {
	current := newMonorepo(t, false)

	buildError := current.run(t, "", false, "build")
	if buildError == nil || !strings.Contains(buildError.Error(), "-t: packages/api, packages/web, tools/a/b/cli") {
		t.Fatalf("at a root with no .shipo, steps ask for -t: %v", buildError)
	}

	app := current.app("", false)
	app.Dir = filepath.Join(current.repo.Work, "packages", "web", "src")

	versionError := app.Dispatch(context.Background(), []string{"version"})
	if versionError != nil || !strings.Contains(current.output.String(), "3.0.0") {
		t.Fatalf("with no terminal, the nearest .shipo is used: %v\n%s", versionError, current.output)
	}
}

func TestSingleTargetFromRoot(t *testing.T) {
	current := newSession(t, "", map[string]string{"package.json": PACKAGE_JSON, "web/package.json": "{\"version\": \"2.0.0\"}\n", "web/.shipo": ""})
	current.repo.Git("rm", "--quiet", ".shipo")
	current.repo.Commit("one target")

	versionError := current.run(t, "", false, "version")
	if versionError != nil || !strings.Contains(current.output.String(), "2.0.0") || strings.Contains(current.output.String(), "target     web") {
		t.Fatalf("one target is used without a menu: %v\n%s", versionError, current.output)
	}
}
