package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/testkit"
)

func TestStepsReleaseAndPush(t *testing.T) {
	current := newSession(t, "[build]\ncommand = \"git --version\"\n", map[string]string{"package.json": PACKAGE_JSON})
	background := context.Background()

	bumpError := current.app("", false).Dispatch(background, []string{"bump", "minor"})
	if bumpError != nil {
		t.Fatalf("bump: %v\n%s", bumpError, current.output)
	}

	if !strings.Contains(current.repo.Read("package.json"), `"version": "1.1.0"`) {
		t.Fatalf("package.json = %s", current.repo.Read("package.json"))
	}

	if subject := current.repo.Git("log", "-1", "--format=%s"); subject != "chore: release v1.1.0" {
		t.Fatalf("subject = %q", subject)
	}

	if tags := current.repo.Git("tag", "--points-at", "HEAD"); tags != "v1.1.0" {
		t.Fatalf("tags = %q", tags)
	}

	buildError := current.app("", false).Dispatch(background, []string{"build"})
	if buildError != nil {
		t.Fatalf("build: %v\n%s", buildError, current.output)
	}

	pushError := current.app("", false).Dispatch(background, []string{"push"})
	if pushError != nil {
		t.Fatalf("push: %v\n%s", pushError, current.output)
	}

	if remoteTag := current.repo.RemoteGit("tag", "--list"); remoteTag != "v1.1.0" {
		t.Fatalf("remote tags = %q", remoteTag)
	}

	if head, remoteHead := current.repo.Git("rev-parse", "HEAD"), current.repo.RemoteGit("rev-parse", "main"); head != remoteHead {
		t.Fatal("remote main is not the release commit")
	}

	current.output.Reset()

	versionError := current.app("", false).Dispatch(background, []string{"version"})
	if versionError != nil || !strings.Contains(current.output.String(), "version  1.1.0") {
		t.Fatalf("version: %v\n%s", versionError, current.output)
	}
}

func TestBumpRefusesDirtyTree(t *testing.T) {
	current := newSession(t, "", map[string]string{"package.json": PACKAGE_JSON})

	writeError := os.WriteFile(filepath.Join(current.repo.Work, "dirty.txt"), []byte("x"), 0o644)
	if writeError != nil {
		t.Fatal(writeError)
	}

	bumpError := current.app("", false).Dispatch(context.Background(), []string{"bump", "patch"})
	if bumpError == nil || !strings.Contains(bumpError.Error(), "tree") {
		t.Fatalf("want a dirty tree error, got %v", bumpError)
	}
}

func TestGitPresetTagsOnly(t *testing.T) {
	current := newSession(t, "[version]\npreset = \"go\"\n", map[string]string{"go.mod": "module example.com/app\n"})

	bumpError := current.app("", false).Dispatch(context.Background(), []string{"bump", "minor"})
	if bumpError != nil {
		t.Fatalf("bump: %v\n%s", bumpError, current.output)
	}

	if subject := current.repo.Git("log", "-1", "--format=%s"); subject != "init" {
		t.Fatalf("a git preset must not commit, got %q", subject)
	}

	if tags := current.repo.Git("tag", "--list"); tags != "v0.1.0" {
		t.Fatalf("tags = %q", tags)
	}
}

func TestCustomCommitAndSubfolderTag(t *testing.T) {
	current := newSession(t, "", map[string]string{"package.json": PACKAGE_JSON})

	subfolder := filepath.Join(current.repo.Work, "web")

	current.repo.Write(map[string]string{
		"web/package.json": "{\"version\": \"2.0.0\"}\n",
		"web/.shipo":       "[bump]\ncommit = \"release: {tag} ({bump})\"\n",
	})
	current.repo.Commit("web")

	app := current.app("", false)
	app.Dir = subfolder

	bumpError := app.Dispatch(context.Background(), []string{"bump", "patch"})
	if bumpError != nil {
		t.Fatalf("bump: %v\n%s", bumpError, current.output)
	}

	if subject := current.repo.Git("log", "-1", "--format=%s"); subject != "release: web/v2.0.1 (patch)" {
		t.Fatalf("subject = %q", subject)
	}

	if !strings.Contains(current.repo.Read("package.json"), `"version": "1.0.0"`) {
		t.Fatal("the root version must not change")
	}
}

func TestStepsNeedARelease(t *testing.T) {
	current := newSession(t, "", nil)

	for _, step := range []string{"build", "push", "release", "workflow", "command"} {
		stepError := current.run(t, "", false, step)
		if !errors.Is(stepError, release.ErrNoRelease) {
			t.Fatalf("%s: want ErrNoRelease, got %v", step, stepError)
		}
	}
}

func TestStepsNotSet(t *testing.T) {
	current := newSession(t, "", nil)
	current.repo.Git("tag", "v1.0.0")

	for _, step := range []string{"build", "release", "workflow", "command"} {
		current.output.Reset()

		if current.run(t, "", false, step) != nil || !strings.Contains(current.output.String(), "not set") {
			t.Fatalf("%s: want a skip, got %s", step, current.output)
		}
	}

	off := newSession(t, "[publish]\npush = false\n", nil)
	off.repo.Git("tag", "v1.0.0")

	if !strings.Contains(off.run(t, "", false, "release").Error(), "needs push") {
		t.Fatal("release with push off must say it needs push")
	}

	off.output.Reset()

	if off.run(t, "", false, "push") != nil || !strings.Contains(off.output.String(), "push       not set") {
		t.Fatalf("push off: %s", off.output)
	}
}

func TestStepsRunTasks(t *testing.T) {
	current := newSession(t, "[build]\ncommand = \"npm run build\"\n\n[publish]\nrelease = true\nworkflow = \"build.yml version={tag}\"\ncommand = \"npm publish\"\n", nil)
	current.repo.Git("tag", "v1.0.0")
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{
		"gh release view": {Error: testkit.ErrFailed},
		"gh workflow run": {Output: "https://github.com/o/r/actions/runs/3"},
	})

	for _, step := range [][]string{{"build"}, {"push"}, {"release"}, {"workflow", "--no-wait"}, {"command"}} {
		stepError := current.run(t, "", false, step...)
		if stepError != nil {
			t.Fatalf("%v: %v\n%s", step, stepError, current.output)
		}
	}

	for _, call := range []string{"npm run build", "gh release create v1.0.0", "gh workflow run build.yml --ref main --raw-field version=v1.0.0", "npm publish"} {
		if !current.runner.Called(call) {
			t.Fatalf("missing call %q in:\n%s", call, strings.Join(current.runner.Calls(), "\n"))
		}
	}

	if current.runner.Called("gh run watch") {
		t.Fatal("--no-wait must not watch")
	}

	if current.repo.RemoteGit("tag") != "v1.0.0" {
		t.Fatal("push step did not push")
	}
}

func TestStepChecksFail(t *testing.T) {
	current := newSession(t, "[build]\ndir = \"nope\"\ncommand = \"npm run build\"\n\n[publish]\ncommand = \"shipo-missing-tool\"\n", nil)
	current.repo.Git("tag", "v1.0.0")

	if current.run(t, "", false, "build") == nil {
		t.Fatal("build check must fail")
	}

	current.output.Reset()

	commandError := current.run(t, "", false, "command")
	if commandError == nil || !strings.Contains(current.output.String(), "✗ tools     not installed: shipo-missing-tool") {
		t.Fatalf("command check must fail and say why, got %v:\n%s", commandError, current.output)
	}
}

func TestStepBumpChecks(t *testing.T) {
	current := newSession(t, "", nil)
	current.repo.Git("tag", "v1.0.1")

	bumpError := current.run(t, "", false, "bump", "patch")
	if bumpError == nil || !strings.Contains(current.output.String(), "✗ tag       v1.0.1 already exists") {
		t.Fatalf("want a taken tag, got %v:\n%s", bumpError, current.output)
	}
}

func TestStepReleaseNeedsRemoteTag(t *testing.T) {
	current := newSession(t, "[publish]\nrelease = true\nworkflow = \"build.yml\"\n", nil)
	current.repo.Git("tag", "v1.0.0")

	for _, step := range []string{"release", "workflow"} {
		stepError := current.run(t, "", false, step)
		if stepError == nil || !strings.Contains(stepError.Error(), "tag v1.0.0 is not on origin; run shipo push first") {
			t.Fatalf("%s: want a remote tag error, got %v", step, stepError)
		}
	}

	if current.runner.Called("gh release create") || current.runner.Called("gh workflow run") {
		t.Fatal("nothing must run before the tag is pushed")
	}
}
