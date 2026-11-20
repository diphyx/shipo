package cli

import (
	"context"
	"strings"
	"testing"
)

const STABLE = "[bump]\nsuffix = \"next\"\n\n[publish]\npush = \"origin/{branch}\"\n"

func TestFlowStableFinal(t *testing.T) {
	current := newSession(t, STABLE, map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo

	repo.Git("checkout", "--quiet", "-b", "next")
	repo.Write(map[string]string{"package.json": `{"name": "app", "version": "1.1.0-next"}`})
	repo.Commit("release next")
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

	repo.Git("checkout", "--quiet", "-b", "next")
	repo.Write(map[string]string{"package.json": `{"name": "app", "version": "1.1.0-next"}`})
	repo.Commit("release next")

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

	repo.Git("checkout", "--quiet", "-b", "next")
	repo.Write(map[string]string{"package.json": `{"name": "app", "version": "1.1.0-next"}`})
	repo.Commit("release next")
	repo.Write(map[string]string{"notes.txt": "wip"})

	flowError := current.app("", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil || !strings.Contains(flowError.Error(), "commit or stash") {
		t.Fatalf("want a dirty tree error, got %v", flowError)
	}

	if branch := repo.Git("branch", "--show-current"); branch != "next" {
		t.Fatalf("on %q, want next", branch)
	}
}
