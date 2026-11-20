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

// TestFlowStableFinalTaken stops final before the switch when its tag is
// already out.
func TestFlowStableFinalTaken(t *testing.T) {
	current := newSession(t, "[version]\npreset = \"git\"\n\n"+STABLE, nil)
	repo := current.repo

	repo.Git("tag", "v1.2.0")
	onNext(current, map[string]string{"feature.txt": "new"})
	repo.Git("tag", "v1.2.0-next")

	flowError := current.app("n\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil || !strings.Contains(flowError.Error(), "v1.2.0 already exists") {
		t.Fatalf("want a taken-tag error, got %v\n%s", flowError, current.output)
	}

	if strings.Contains(current.output.String(), "switch") {
		t.Fatalf("switched before stopping:\n%s", current.output)
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
	if flowError == nil || !strings.Contains(flowError.Error(), "is not above 1.3.0") {
		t.Fatalf("want a not-above error, got %v\n%s", flowError, current.output)
	}

	if repo.Git("rev-parse", "main") != main || repo.Git("branch", "--show-current") != "next" {
		t.Fatal("main was not reset, or not back on next")
	}
}
