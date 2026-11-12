package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/ui"
)

func TestFlowNeedsTerminal(t *testing.T) {
	current := newSession(t, "", map[string]string{"package.json": PACKAGE_JSON})

	flowError := current.app("", false).Dispatch(context.Background(), []string{"patch"})
	if flowError != ui.ErrNoTerminal {
		t.Fatalf("want ErrNoTerminal, got %v", flowError)
	}
}

func TestFlowRevertsOnBuildFailure(t *testing.T) {
	current := newSession(t, "[build]\ncommand = \"git no-such-command\"\n", map[string]string{"package.json": PACKAGE_JSON})
	before := current.repo.Git("rev-parse", "HEAD")

	flowError := current.app("1\n", true).Dispatch(context.Background(), nil)
	if flowError == nil {
		t.Fatalf("want a build error\n%s", current.output)
	}

	if after := current.repo.Git("rev-parse", "HEAD"); after != before {
		t.Fatal("HEAD was not reset")
	}

	if tags := current.repo.Git("tag", "--list"); tags != "" {
		t.Fatalf("tag not deleted: %q", tags)
	}

	if !strings.Contains(current.repo.Read("package.json"), `"version": "1.0.0"`) {
		t.Fatal("version file not restored")
	}
}

func TestFlowSkipsDeclinedCommand(t *testing.T) {
	current := newSession(t, "[bump]\nlevel = \"patch\"\n\n[publish]\ncommand = \"git --version\"\n", map[string]string{"package.json": PACKAGE_JSON})

	flowError := current.app("n\n", true).Dispatch(context.Background(), nil)
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	if !strings.Contains(current.output.String(), "command    skipped") {
		t.Fatalf("command was not skipped:\n%s", current.output)
	}

	if remoteTag := current.repo.RemoteGit("tag", "--list"); remoteTag != "v1.0.1" {
		t.Fatalf("remote tags = %q", remoteTag)
	}
}

func TestFlowFixedLevelAndConfirms(t *testing.T) {
	current := newSession(t, "[bump]\nlevel = \"minor\"\n\n[publish]\nrelease = true\nworkflow = \"build.yml\"\ncommand = \"npm publish\"\n", nil)
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{
		"gh release view": {Error: testkit.ErrFailed},
		"gh workflow run": {Output: "https://github.com/o/r/actions/runs/8"},
	})

	flowError := current.run(t, "y\nn\ny\n", true)
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	text := current.output.String()

	for _, line := range []string{"✓ tag        v1.1.0", "Create? [y/N]", "✓ release    v1.1.0", "- workflow   skipped", "✓ command    [1/1] npm publish"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	if strings.Contains(text, "1) patch") {
		t.Fatal("a fixed level must not show the menu")
	}

	if current.runner.Called("gh workflow run") {
		t.Fatal("a declined workflow must not run")
	}
}

func TestFlowBadLevel(t *testing.T) {
	current := newSession(t, "[bump]\nlevel = \"huge\"\n", nil)

	flowError := current.run(t, "", true)
	if flowError == nil || !strings.Contains(flowError.Error(), "bump.level") {
		t.Fatalf("want a bump.level error, got %v", flowError)
	}
}

func TestFlowChecksStopBeforeChanges(t *testing.T) {
	current := newSession(t, "[bump]\nlevel = \"patch\"\n", nil)
	current.repo.Write(map[string]string{"dirty.txt": "x"})
	head := current.repo.Git("rev-parse", "HEAD")

	if current.run(t, "", true) == nil {
		t.Fatal("want a dirty tree error")
	}

	if current.repo.Git("rev-parse", "HEAD") != head || current.repo.Git("tag") != "" {
		t.Fatal("a failed check must change nothing")
	}
}

func TestFlowPublishFailureKeepsRelease(t *testing.T) {
	current := newSession(t, "[bump]\nlevel = \"patch\"\n\n[publish]\ncommand = \"npm publish\"\n", nil)
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{"npm publish": {Error: testkit.ErrFailed}})

	if current.run(t, "y\n", true) == nil {
		t.Fatal("want the npm error")
	}

	if current.repo.Git("tag") != "v1.0.1" || current.repo.RemoteGit("tag") != "v1.0.1" {
		t.Fatal("publish must never revert")
	}

	if !strings.Contains(current.output.String(), "failed; nothing was undone") {
		t.Fatalf("output:\n%s", current.output)
	}
}

func TestFlowMissingReleaseFilesRevertsBump(t *testing.T) {
	current := newSession(t, "[bump]\nlevel = \"patch\"\n\n[publish]\nrelease = \"dist/*\"\n", nil)
	head := current.repo.Git("rev-parse", "HEAD")

	if current.run(t, "", true) == nil {
		t.Fatal("want a missing files error")
	}

	if current.repo.Git("rev-parse", "HEAD") != head || current.repo.Git("tag") != "" || current.repo.RemoteGit("tag") != "" {
		t.Fatal("missing release files must revert the bump before anything is sent")
	}
}

func TestFlowRetry(t *testing.T) {
	current := newSession(t, "[bump]\nlevel = \"patch\"\n", nil)
	current.repo.Write(map[string]string{"package.json": "{\n  \"name\": \"app\",\n  \"version\": \"1.0.1\"\n}\n"})
	current.repo.Commit("chore: release v1.0.1")
	current.repo.Git("tag", "v1.0.1")

	if current.run(t, "y\n", true) != nil {
		t.Fatalf("retry failed:\n%s", current.output)
	}

	if !strings.Contains(current.output.String(), "HEAD is release v1.0.1, not yet pushed") || current.repo.RemoteGit("tag") != "v1.0.1" {
		t.Fatalf("retry did not publish:\n%s", current.output)
	}

	if current.repo.Git("tag") != "v1.0.1" {
		t.Fatal("retry must not bump again")
	}
}

func TestFlowRetryDeclined(t *testing.T) {
	current := newSession(t, "[bump]\nlevel = \"patch\"\n", nil)
	current.repo.Write(map[string]string{"package.json": "{\n  \"name\": \"app\",\n  \"version\": \"1.0.1\"\n}\n"})
	current.repo.Commit("chore: release v1.0.1")
	current.repo.Git("tag", "v1.0.1")

	if current.run(t, "n\n", true) != nil {
		t.Fatalf("flow failed:\n%s", current.output)
	}

	if current.repo.Git("tag", "--points-at", "HEAD") != "v1.0.2" {
		t.Fatal("declining the retry must start a new release")
	}
}
