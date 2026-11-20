package cli

import (
	"context"
	"strings"
	"testing"
)

const STABLE = "[bump]\nsuffix = \"next\"\n\n[publish]\npush = \"origin/{branch}\"\n"

// onNext starts next from main with one commit of files.
func onNext(current *session, files map[string]string) {
	current.repo.Git("checkout", "--quiet", "-b", "next")
	current.repo.Write(files)
	current.repo.Commit("next")
}

// NEXT_PACKAGE is package.json on next, at a pre-release.
const NEXT_PACKAGE = `{"name": "app", "version": "1.1.0-next"}`

func TestFlowStableFinal(t *testing.T) {
	current := newSession(t, STABLE, map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo

	onNext(current, map[string]string{"package.json": NEXT_PACKAGE})
	repo.Git("tag", "v1.1.0-next")
	repo.Git("push", "--quiet", "origin", "next", "v1.1.0-next")

	flowError := current.app("", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	text := current.output.String()
	for _, line := range []string{"✓ switch     main", "✓ merge      next (1 commit)", "✓ tag        v1.1.0", "✓ update     next ← main", "✓ switch     next"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	if branch := repo.Git("branch", "--show-current"); branch != "next" {
		t.Fatalf("back on %q, want next", branch)
	}

	if repo.Git("rev-parse", "next") != repo.Git("rev-parse", "main") {
		t.Fatal("next was not brought up to main")
	}

	if remote := repo.RemoteGit("rev-parse", "main"); remote != repo.Git("rev-parse", "main") {
		t.Fatal("main was not pushed")
	}

	if remote := repo.RemoteGit("rev-parse", "next"); remote != repo.Git("rev-parse", "next") {
		t.Fatal("next was not pushed")
	}

	if !strings.Contains(repo.Read("package.json"), `"version": "1.1.0"`) {
		t.Fatalf("package.json = %s", repo.Read("package.json"))
	}
}

func TestFlowStableUndo(t *testing.T) {
	current := newSession(t, STABLE+"\n[build]\ncommand = \"git no-such-command\"\n", map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo
	main := repo.Git("rev-parse", "main")

	onNext(current, map[string]string{"package.json": NEXT_PACKAGE})

	flowError := current.app("", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil {
		t.Fatalf("want a build error\n%s", current.output)
	}

	if branch := repo.Git("branch", "--show-current"); branch != "next" {
		t.Fatalf("back on %q, want next", branch)
	}

	if repo.Git("rev-parse", "main") != main {
		t.Fatal("main was not reset")
	}

	if tags := repo.Git("tag", "--list"); tags != "" {
		t.Fatalf("tags left: %q", tags)
	}
}

func TestFlowStableDirtyTree(t *testing.T) {
	current := newSession(t, STABLE, map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo

	onNext(current, map[string]string{"package.json": NEXT_PACKAGE})
	repo.Write(map[string]string{"notes.txt": "wip"})

	flowError := current.app("", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil || !strings.Contains(flowError.Error(), "commit or stash") {
		t.Fatalf("want a dirty tree error, got %v", flowError)
	}

	if branch := repo.Git("branch", "--show-current"); branch != "next" {
		t.Fatalf("on %q, want next", branch)
	}
}

// TestFlowStableFinalGitTags covers git presets: main only reads its own
// tags, so final takes the pre-release from next before the switch.
func TestFlowStableFinalGitTags(t *testing.T) {
	current := newSession(t, "[version]\npreset = \"git\"\n\n"+STABLE, nil)
	repo := current.repo

	repo.Git("tag", "v1.1.0")
	onNext(current, map[string]string{"feature.txt": "new"})
	repo.Git("tag", "v1.2.0-next")
	repo.Git("push", "--quiet", "origin", "next", "v1.1.0", "v1.2.0-next")

	flowError := current.app("", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	if !strings.Contains(current.output.String(), "✓ tag        v1.2.0") {
		t.Fatalf("no v1.2.0 tag:\n%s", current.output)
	}

	if tagged := repo.Git("rev-parse", "v1.2.0^{commit}"); tagged != repo.Git("rev-parse", "main") {
		t.Fatal("v1.2.0 is not on main")
	}

	if branch := repo.Git("branch", "--show-current"); branch != "next" {
		t.Fatalf("back on %q, want next", branch)
	}
}

// TestFlowStableFinalRetag releases final again when its tag is out
// already, as current does: the tag moves to the new release commit, on the
// remote too.
func TestFlowStableFinalRetag(t *testing.T) {
	current := newSession(t, "[version]\npreset = \"git\"\n\n"+STABLE, nil)
	repo := current.repo

	repo.Git("tag", "v1.0.0")
	repo.Write(map[string]string{"fix.txt": "fix"})
	repo.Commit("fix")
	repo.Git("tag", "v1.2.0")
	repo.Git("push", "--quiet", "origin", "main", "v1.0.0", "v1.2.0")

	onNext(current, map[string]string{"feature.txt": "new"})
	repo.Git("tag", "v1.2.0-next")

	flowError := current.app("n\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	text := current.output.String()
	for _, line := range []string{"✓ bump       final · 1.2.0 → 1.2.0, again", "✓ tag        v1.2.0 moved to HEAD"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	main := repo.Git("rev-parse", "main")
	if repo.Git("rev-parse", "v1.2.0^{commit}") != main || repo.RemoteGit("rev-parse", "v1.2.0^{commit}") != main {
		t.Fatal("v1.2.0 was not moved to main, locally and on the remote")
	}
}

// TestFlowStableFinalNotAbove stops final when main already released past
// the pre-release, and undoes the switch.
func TestFlowStableFinalNotAbove(t *testing.T) {
	current := newSession(t, "[version]\npreset = \"git\"\n\n"+STABLE, nil)
	repo := current.repo
	main := repo.Git("rev-parse", "main")

	repo.Git("tag", "v1.3.0")
	onNext(current, map[string]string{"feature.txt": "new"})
	repo.Git("tag", "v1.2.0-next")

	flowError := current.app("n\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil || !strings.Contains(flowError.Error(), "is below 1.3.0") {
		t.Fatalf("want a below error, got %v\n%s", flowError, current.output)
	}

	if repo.Git("rev-parse", "main") != main || repo.Git("branch", "--show-current") != "next" {
		t.Fatal("main was not reset, or not back on next")
	}
}

// TestFlowStableFinalNoChange re-releases final when the files already hold
// its version, as after an earlier final: no commit, the tag just moves.
func TestFlowStableFinalNoChange(t *testing.T) {
	// The lock file is staged after its refresh, even when it stays the same.
	current := newSession(t, STABLE, map[string]string{"package.json": `{"name": "app", "version": "1.1.0"}`, "package-lock.json": "{}\n"})
	repo := current.repo

	repo.Git("tag", "v1.1.0")
	repo.Git("push", "--quiet", "origin", "main", "v1.1.0")
	onNext(current, map[string]string{"feature.txt": "new"})

	flowError := current.app("n\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	text := current.output.String()
	for _, line := range []string{"- commit     nothing changed", "✓ tag        v1.1.0 moved to HEAD"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	if repo.Git("rev-parse", "v1.1.0^{commit}") != repo.Git("rev-parse", "main") {
		t.Fatal("v1.1.0 is not on main")
	}
}

// joinSession makes main hold a commit next lacks, with next's pre-release
// tagged and pushed.
func joinSession(t *testing.T, shipo string) *session {
	t.Helper()

	current := newSession(t, STABLE+shipo, map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo

	onNext(current, map[string]string{"package.json": NEXT_PACKAGE})
	repo.Git("tag", "v1.1.0-next")
	repo.Git("push", "--quiet", "-u", "origin", "next", "v1.1.0-next")

	repo.Git("checkout", "--quiet", "main")
	repo.Write(map[string]string{"hotfix.txt": "fix"})
	repo.Commit("hotfix on main")
	repo.Git("push", "--quiet", "origin", "main")
	repo.Git("checkout", "--quiet", "next")

	return current
}

// staleMain leaves a commit on origin/main that the local main lacks, as
// before a pull, with next tagged at a pre-release.
func staleMain(t *testing.T, shipo string, files map[string]string, tags ...string) *session {
	t.Helper()

	current := newSession(t, shipo, files)
	repo := current.repo

	for _, tag := range tags {
		repo.Git("tag", tag)
	}

	repo.Git("push", "--quiet", "origin", "main", "--tags")
	onNext(current, map[string]string{"feature.txt": "new"})
	repo.Git("tag", "v1.2.0-next")
	repo.Git("push", "--quiet", "-u", "origin", "next", "v1.2.0-next")

	repo.Git("checkout", "--quiet", "main")
	repo.Write(map[string]string{"hotfix.txt": "fix"})
	repo.Commit("hotfix on main")
	repo.Git("push", "--quiet", "origin", "main")
	repo.Git("reset", "--quiet", "--hard", "HEAD~1")
	repo.Git("checkout", "--quiet", "next")

	return current
}
