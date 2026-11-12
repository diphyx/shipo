package bump

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/ui"
)

const PACKAGE = "{\n  \"name\": \"app\",\n  \"version\": \"1.0.0\"\n}\n"

func plan(t *testing.T, repo *testkit.Repo, runner *testkit.Runner, level semver.Level) *release.Release {
	t.Helper()

	target, openError := release.Open(context.Background(), runner, repo.Work)
	if openError != nil {
		t.Fatal(openError)
	}

	planned, planError := release.NewRelease(context.Background(), target, level)
	if planError != nil {
		t.Fatal(planError)
	}

	return planned
}

func terminal() (*ui.Terminal, *bytes.Buffer) {
	var output bytes.Buffer

	return ui.NewTestTerminal(&output, "", false), &output
}

func TestCreateFile(t *testing.T) {
	repo := testkit.NewRepo(t, "[bump]\ncommit = \"release: {tag} ({bump}) from {previous}\"\n", map[string]string{"package.json": PACKAGE})
	output, printed := terminal()

	result, createError := Create(context.Background(), plan(t, repo, testkit.NewRunner(), semver.LEVEL_MINOR), output)
	if createError != nil {
		t.Fatal(createError)
	}

	if !result.IsCommitted || !result.IsTagged || result.Tag != "v1.1.0" || len(result.Base) != 40 {
		t.Fatalf("result = %+v", result)
	}

	if subject := repo.Git("log", "-1", "--format=%s"); subject != "release: v1.1.0 (minor) from 1.0.0" {
		t.Fatalf("subject = %q", subject)
	}

	if tags := repo.Git("tag", "--points-at", "HEAD"); tags != "v1.1.0" {
		t.Fatalf("tags = %q", tags)
	}

	if !strings.Contains(repo.Read("package.json"), `"version": "1.1.0"`) {
		t.Fatal("version not written")
	}

	if status := repo.Git("status", "--porcelain"); status != "" {
		t.Fatalf("tree not clean after bump: %s", status)
	}

	for _, line := range []string{"✓ version    package.json → 1.1.0", "✓ commit     release: v1.1.0", "✓ tag        v1.1.0"} {
		if !strings.Contains(printed.String(), line) {
			t.Fatalf("output misses %q:\n%s", line, printed)
		}
	}
}

func TestCreateHashInCommitIsLeftAsWritten(t *testing.T) {
	repo := testkit.NewRepo(t, "[bump]\ncommit = \"release {tag} {hash}\"\n", map[string]string{"package.json": PACKAGE})
	output, _ := terminal()

	_, createError := Create(context.Background(), plan(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), output)
	if createError != nil {
		t.Fatal(createError)
	}

	if subject := repo.Git("log", "-1", "--format=%s"); subject != "release v1.0.1 {hash}" {
		t.Fatalf("subject = %q", subject)
	}
}

func TestCreateGitTagOnly(t *testing.T) {
	repo := testkit.NewRepo(t, "[version]\npreset = \"git\"\npattern = \"release-{version}\"\n", nil)
	output, _ := terminal()

	result, createError := Create(context.Background(), plan(t, repo, testkit.NewRunner(), semver.LEVEL_MAJOR), output)
	if createError != nil {
		t.Fatal(createError)
	}

	if result.IsCommitted || !result.IsTagged {
		t.Fatalf("result = %+v", result)
	}

	if subject := repo.Git("log", "-1", "--format=%s"); subject != "init" {
		t.Fatalf("a git source must not commit, got %q", subject)
	}

	if tags := repo.Git("tag"); tags != "release-1.0.0" {
		t.Fatalf("tags = %q", tags)
	}
}

func TestCreateCurrentKeepsTagOnHead(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	repo.Git("tag", "--annotate", "v1.0.0", "--message", "Release v1.0.0")
	head := repo.Git("rev-parse", "HEAD")
	output, printed := terminal()

	result, createError := Create(context.Background(), plan(t, repo, testkit.NewRunner(), semver.LEVEL_CURRENT), output)
	if createError != nil {
		t.Fatal(createError)
	}

	if result.IsCommitted || result.IsTagged || result.MovedFrom != "" || repo.Git("rev-parse", "HEAD") != head {
		t.Fatalf("a tag already on HEAD must stay: %+v", result)
	}

	if !strings.Contains(printed.String(), "- version    1.0.0 (current)") || !strings.Contains(printed.String(), "- tag        v1.0.0 (on HEAD)") {
		t.Fatalf("output:\n%s", printed)
	}
}

func TestCreateCurrentTagsHead(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	output, _ := terminal()

	result, createError := Create(context.Background(), plan(t, repo, testkit.NewRunner(), semver.LEVEL_CURRENT), output)
	if createError != nil {
		t.Fatal(createError)
	}

	if !result.IsTagged || result.IsCommitted || repo.Git("tag", "--points-at", "HEAD") != "v1.0.0" {
		t.Fatalf("a missing tag is created on HEAD, with no commit: %+v", result)
	}
}

func TestCreateCurrentMovesTag(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	old := repo.Git("rev-parse", "HEAD")
	repo.Git("tag", "--annotate", "v1.0.0", "--message", "Release v1.0.0")
	repo.Write(map[string]string{"fix.txt": "fixed\n"})
	repo.Commit("fix: the release")
	head := repo.Git("rev-parse", "HEAD")
	output, printed := terminal()

	planned := plan(t, repo, testkit.NewRunner(), semver.LEVEL_CURRENT)

	result, createError := Create(context.Background(), planned, output)
	if createError != nil {
		t.Fatal(createError)
	}

	if result.MovedFrom != old || repo.Git("rev-parse", "v1.0.0^{commit}") != head {
		t.Fatalf("the tag must move to HEAD: %+v", result)
	}

	if !strings.Contains(printed.String(), "✓ tag        v1.0.0 moved to HEAD") {
		t.Fatalf("output:\n%s", printed)
	}

	revertError := Revert(context.Background(), planned.Target, result)
	if revertError != nil {
		t.Fatal(revertError)
	}

	if repo.Git("rev-parse", "v1.0.0^{commit}") != old || repo.Git("rev-parse", "HEAD") != head {
		t.Fatal("the undo must put the tag back where it was, and keep HEAD")
	}
}

func TestCreateRefreshesLock(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE, "package-lock.json": PACKAGE})
	runner := testkit.NewRunner("npm")
	output, _ := terminal()

	_, createError := Create(context.Background(), plan(t, repo, runner, semver.LEVEL_PATCH), output)
	if createError != nil {
		t.Fatal(createError)
	}

	if !runner.Called("npm install --package-lock-only --ignore-scripts") {
		t.Fatalf("lock not refreshed: %v", runner.Calls())
	}
}

func TestCreateRevertsOnLockFailure(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE, "yarn.lock": "# lock\n"})
	head := repo.Git("rev-parse", "HEAD")

	runner := testkit.NewRunner("yarn")
	runner.Handle = testkit.WithReplies(map[string]testkit.Response{"yarn": {Error: testkit.ErrFailed}})

	output, _ := terminal()

	_, createError := Create(context.Background(), plan(t, repo, runner, semver.LEVEL_PATCH), output)
	if createError == nil || !strings.Contains(createError.Error(), "refresh yarn.lock") {
		t.Fatalf("want a lock error, got %v", createError)
	}

	if repo.Git("rev-parse", "HEAD") != head || repo.Git("tag") != "" || !strings.Contains(repo.Read("package.json"), `"1.0.0"`) {
		t.Fatal("bump was not reverted")
	}
}

func TestCreateRevertsWhenTagExists(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	head := repo.Git("rev-parse", "HEAD")
	repo.Git("tag", "v1.0.1", "HEAD")

	output, _ := terminal()

	_, createError := Create(context.Background(), plan(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), output)
	if createError == nil {
		t.Fatal("want an error when the tag exists")
	}

	if repo.Git("rev-parse", "HEAD") != head || !strings.Contains(repo.Read("package.json"), `"1.0.0"`) {
		t.Fatal("the commit was not reverted")
	}

	if repo.Git("tag") != "v1.0.1" {
		t.Fatal("an existing tag must never be deleted")
	}
}

func TestRevert(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	output, _ := terminal()

	planned := plan(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH)

	result, createError := Create(context.Background(), planned, output)
	if createError != nil {
		t.Fatal(createError)
	}

	revertError := Revert(context.Background(), planned.Target, result)
	if revertError != nil {
		t.Fatal(revertError)
	}

	if repo.Git("rev-parse", "HEAD") != result.Base || repo.Git("tag") != "" || repo.Git("status", "--porcelain") != "" {
		t.Fatal("Revert did not restore the base")
	}
}

func TestRevertAfterCtrlC(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	output, _ := terminal()

	planned := plan(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH)

	result, createError := Create(context.Background(), planned, output)
	if createError != nil {
		t.Fatal(createError)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	revertError := Revert(cancelled, planned.Target, result)
	if revertError != nil {
		t.Fatalf("the undo must finish after Ctrl-C: %v", revertError)
	}

	if repo.Git("rev-parse", "HEAD") != result.Base || repo.Git("tag") != "" {
		t.Fatal("the bump was not undone")
	}
}
