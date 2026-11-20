package cli

import (
	"context"
	"strings"
	"testing"
)

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

func TestFlowStableJoinMerge(t *testing.T) {
	current := joinSession(t, "")
	repo := current.repo

	flowError := current.app("1\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	text := current.output.String()
	for _, line := range []string{"main has 1 commit next lacks", "rebase   → next on top of", "✓ merge      next (1 commit) + main's 1"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	if parents := strings.Fields(repo.Git("log", "-1", "--format=%p", "main~1")); len(parents) != 2 {
		t.Fatalf("want a merge commit under the release, got parents %v", parents)
	}

	if repo.Git("rev-parse", "next") != repo.Git("rev-parse", "main") {
		t.Fatal("next was not brought up to main")
	}
}

func TestFlowStableJoinRebase(t *testing.T) {
	current := joinSession(t, "")
	repo := current.repo

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	text := current.output.String()
	for _, line := range []string{"✓ rebase     next (1 commit) onto main", "force-push to origin", "- tag        v1.1.0-next stays on the old next"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	if merges := repo.Git("log", "--merges", "--format=%h", "main"); merges != "" {
		t.Fatalf("rebase must leave no merge commit, got %q", merges)
	}

	if repo.RemoteGit("rev-parse", "next") != repo.Git("rev-parse", "main") {
		t.Fatal("next was not force-pushed to the release")
	}
}

func TestFlowStableJoinRebaseUndo(t *testing.T) {
	current := joinSession(t, "\n[build]\ncommand = \"git no-such-command\"\n")
	repo := current.repo
	next := repo.Git("rev-parse", "next")
	main := repo.Git("rev-parse", "main")

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil {
		t.Fatalf("want a build error\n%s", current.output)
	}

	if !strings.Contains(current.output.String(), "main and next undone") {
		t.Fatalf("no undo line:\n%s", current.output)
	}

	if repo.Git("rev-parse", "next") != next || repo.Git("rev-parse", "main") != main || repo.Git("branch", "--show-current") != "next" {
		t.Fatal("next and main were not put back, or not on next")
	}
}

// TestFlowStableRemoteAhead keeps a push someone else made to next: final
// fetches first, and stops before any change when next lacks it.
func TestFlowStableRemoteAhead(t *testing.T) {
	current := joinSession(t, "")
	repo := current.repo
	main := repo.Git("rev-parse", "main")

	// Someone moves next on the remote; the local view of it stays old.
	theirs := repo.Git("rev-parse", "main")
	repo.RemoteGit("update-ref", "refs/heads/next", theirs)

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil || !strings.Contains(flowError.Error(), "origin/next has commits you don't have") {
		t.Fatalf("want a pull-first error, got %v\n%s", flowError, current.output)
	}

	if repo.RemoteGit("rev-parse", "next") != theirs || repo.Git("rev-parse", "main") != main {
		t.Fatal("something changed before the stop")
	}
}

// TestFlowStableJoinStaleMain asks too when only origin/main has the
// commits next lacks: final fetches before it counts.
func TestFlowStableJoinStaleMain(t *testing.T) {
	current := joinSession(t, "")
	repo := current.repo

	// Local main falls behind origin/main, as before a fetch.
	repo.Git("branch", "--force", "main", "main~1")
	repo.Git("update-ref", "-d", "refs/remotes/origin/main")

	flowError := current.app("1\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	if !strings.Contains(current.output.String(), "main has 1 commit next lacks") {
		t.Fatalf("no join question:\n%s", current.output)
	}
}

// TestFlowStableJoinRemoteOnly asks too when main exists only on the
// remote: its commits are counted from origin/main, which git switch makes
// main from.
func TestFlowStableJoinRemoteOnly(t *testing.T) {
	current := joinSession(t, "")
	repo := current.repo

	repo.Git("branch", "--quiet", "-D", "main")

	flowError := current.app("1\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	if !strings.Contains(current.output.String(), "main has 1 commit next lacks") {
		t.Fatalf("no join question:\n%s", current.output)
	}
}
