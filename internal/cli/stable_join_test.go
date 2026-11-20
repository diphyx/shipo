package cli

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFlowStableJoinMerge(t *testing.T) {
	current := joinSession(t, "")
	repo := current.repo

	flowError := current.app("1\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	text := current.output.String()
	for _, line := range []string{"main has 1 commit next lacks", "replace  → main becomes next", "✓ merge      next (1 commit) + main's 1"} {
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

// TestFlowStableJoinReplace releases exactly next: main's extra commit is
// listed, then dropped from main, which is pushed with a lease.
func TestFlowStableJoinReplace(t *testing.T) {
	current := joinSession(t, "")
	repo := current.repo
	hotfix := repo.Git("rev-parse", "--short", "main")
	old := repo.Git("rev-parse", "main")
	backup := "backup/main-" + time.Now().Format("2006-01-02")

	// An earlier backup of the day keeps its name.
	earlier := repo.Git("rev-parse", "main~1")
	repo.Git("branch", backup, earlier)

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	if repo.Git("rev-parse", backup+"-2") != old || repo.RemoteGit("rev-parse", backup+"-2") != old {
		t.Fatalf("main was not backed up as %s-2, locally and on the remote:\n%s", backup, current.output)
	}

	if repo.Git("rev-parse", backup) != earlier {
		t.Fatal("the earlier backup was overwritten")
	}

	text := current.output.String()
	for _, line := range []string{"replace  → main becomes next; drops " + hotfix, "- drop       1 commit of main, kept in " + backup + "-2", "✓ replace    main = next"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	if strings.Contains(repo.Git("log", "--format=%s", "main"), "hotfix on main") {
		t.Fatal("main still holds the dropped commit")
	}

	if repo.RemoteGit("rev-parse", "main") != repo.Git("rev-parse", "main") || repo.Git("rev-parse", "next") != repo.Git("rev-parse", "main") {
		t.Fatal("main was not pushed, or next not brought up to it")
	}
}
func TestFlowStableJoinReplaceUndo(t *testing.T) {
	current := joinSession(t, "\n[build]\ncommand = \"git no-such-command\"\n")
	repo := current.repo
	next := repo.Git("rev-parse", "next")
	main := repo.Git("rev-parse", "main")

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil {
		t.Fatalf("want a build error\n%s", current.output)
	}

	if repo.Git("rev-parse", "next") != next || repo.Git("rev-parse", "main") != main || repo.Git("branch", "--show-current") != "next" {
		t.Fatal("main was not put back, or not on next")
	}

	if repo.RemoteGit("rev-parse", "main") != main {
		t.Fatal("the remote main changed")
	}
}

// TestFlowStableReplaceDiverged lists and drops the commits of both mains
// when the local one and the remote one diverged.
func TestFlowStableReplaceDiverged(t *testing.T) {
	current := joinSession(t, "")
	repo := current.repo

	// The remote main keeps "hotfix on main"; the local main has its own.
	repo.Git("checkout", "--quiet", "main")
	repo.Git("reset", "--quiet", "--hard", "main~1")
	repo.Write(map[string]string{"local.txt": "local"})
	repo.Commit("local only")
	repo.Git("checkout", "--quiet", "next")

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	backup := "backup/main-" + time.Now().Format("2006-01-02")
	if repo.RemoteGit("log", "-1", "--format=%s", backup) != "hotfix on main" || repo.RemoteGit("log", "-1", "--format=%s", backup+"-local") != "local only" {
		t.Fatalf("both mains were not backed up:\n%s", current.output)
	}

	text := current.output.String()
	for _, line := range []string{"main has 2 commits next lacks", "- drop       2 commits of main, kept in " + backup + " and " + backup + "-local"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	if repo.RemoteGit("rev-parse", "main") != repo.Git("rev-parse", "main") {
		t.Fatal("the remote main was not replaced")
	}
}

// TestFlowStableReplaceUndoRemoteOnly removes the main the switch made, so
// a failed replace leaves no local main equal to next behind.
func TestFlowStableReplaceUndoRemoteOnly(t *testing.T) {
	current := joinSession(t, "\n[build]\ncommand = \"git no-such-command\"\n")
	repo := current.repo

	repo.Git("branch", "--quiet", "-D", "main")

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil {
		t.Fatalf("want a build error\n%s", current.output)
	}

	if branches := repo.Git("branch", "--list", "main"); branches != "" {
		t.Fatalf("the main the switch made is still there: %q", branches)
	}
}

// TestFlowStableMergeConflict stops a merge that conflicts before any
// change, even with no local main, and leaves no merge halfway.
func TestFlowStableMergeConflict(t *testing.T) {
	current := newSession(t, STABLE, map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo

	onNext(current, map[string]string{"package.json": NEXT_PACKAGE})
	repo.Git("push", "--quiet", "-u", "origin", "next")
	repo.Git("checkout", "--quiet", "main")
	repo.Write(map[string]string{"package.json": `{"name": "app", "version": "1.0.1"}`})
	repo.Commit("hotfix on main")
	repo.Git("push", "--quiet", "origin", "main")
	repo.Git("checkout", "--quiet", "next")
	repo.Git("branch", "--quiet", "-D", "main")

	flowError := current.app("1\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil || !strings.Contains(flowError.Error(), "conflict") {
		t.Fatalf("want a conflict error, got %v\n%s", flowError, current.output)
	}

	if !strings.Contains(current.output.String(), "merge    → conflicts; merge main into next by hand first") {
		t.Fatalf("the menu doesn't say merge conflicts:\n%s", current.output)
	}

	if branch := repo.Git("branch", "--show-current"); branch != "next" || repo.Git("status", "--short") != "" {
		t.Fatalf("on %q with %q, want a clean next", branch, repo.Git("status", "--short"))
	}
}

// TestFlowStableReplacePastRelease stops a replace below what main already
// released: main's files hold next's version then, so its tags tell.
func TestFlowStableReplacePastRelease(t *testing.T) {
	current := newSession(t, STABLE, map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo

	onNext(current, map[string]string{"package.json": NEXT_PACKAGE})
	repo.Git("push", "--quiet", "-u", "origin", "next")
	repo.Git("checkout", "--quiet", "main")
	repo.Write(map[string]string{"package.json": `{"name": "app", "version": "2.0.0"}`})
	repo.Commit("chore: release v2.0.0")
	repo.Git("tag", "v2.0.0")
	repo.Git("push", "--quiet", "origin", "main", "v2.0.0")
	repo.Git("checkout", "--quiet", "next")
	main := repo.Git("rev-parse", "main")

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError == nil || !strings.Contains(flowError.Error(), "is below 2.0.0") {
		t.Fatalf("want a below error, got %v\n%s", flowError, current.output)
	}

	if repo.RemoteGit("rev-parse", "main") != main || repo.Git("rev-parse", "main") != main {
		t.Fatal("main changed")
	}
}
