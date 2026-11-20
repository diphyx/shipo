package cli

import (
	"context"
	"strings"
	"testing"
)

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

// TestFlowStableMergeStaleMain merges a stale local main: it catches up to
// origin/main first, so the bump has nothing left to merge.
func TestFlowStableMergeStaleMain(t *testing.T) {
	cases := map[string][]string{"new release": {"v1.1.0"}, "release again": {"v1.1.0", "v1.2.0"}}

	for name, tags := range cases {
		t.Run(name, func(t *testing.T) {
			current := staleMain(t, "[version]\npreset = \"git\"\n\n"+STABLE, nil, tags...)
			repo := current.repo

			flowError := current.app("1\n", true).Dispatch(context.Background(), []string{"final"})
			if flowError != nil {
				t.Fatalf("flow: %v\n%s", flowError, current.output)
			}

			if !strings.Contains(repo.Git("log", "--format=%s", "main"), "hotfix on main") {
				t.Fatal("main lacks origin/main's commit")
			}

			if repo.RemoteGit("rev-parse", "v1.2.0^{commit}") != repo.Git("rev-parse", "main") {
				t.Fatal("v1.2.0 is not on main")
			}
		})
	}
}

// TestFlowStableTestedReplace is the history of an earlier final that merged
// main's own work into next after its pre-release was tested: final
// releases the tested tag, and replace drops the rest from main.
func TestFlowStableTestedReplace(t *testing.T) {
	current := newSession(t, STABLE, map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo

	repo.Git("tag", "v1.0.0")
	repo.Git("push", "--quiet", "origin", "main", "v1.0.0")

	onNext(current, map[string]string{"package.json": NEXT_PACKAGE})
	repo.Git("tag", "v1.1.0-next")
	repo.Git("push", "--quiet", "-u", "origin", "next", "v1.1.0-next")
	tested := repo.Git("rev-parse", "v1.1.0-next^{commit}")

	// The earlier final: main's own work, merged with next and released,
	// then next brought up to it.
	repo.Git("checkout", "--quiet", "main")
	repo.Write(map[string]string{"schedules.txt": "untested"})
	repo.Commit("feat: schedules")
	repo.Git("merge", "--quiet", "--no-edit", "next")
	repo.Write(map[string]string{"package.json": `{"name": "app", "version": "1.1.0"}`})
	repo.Commit("chore: release v1.1.0")
	repo.Git("tag", "v1.1.0")
	repo.Git("push", "--quiet", "origin", "main", "v1.1.0")
	repo.Git("checkout", "--quiet", "next")
	repo.Git("merge", "--quiet", "--ff-only", "main")
	repo.Git("push", "--quiet", "origin", "next")

	flowError := current.app("2\n", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	text := current.output.String()
	for _, line := range []string{"main has 3 commits v1.1.0-next lacks", "- drop       3 commits of main, kept in backup/main-", "✓ replace    main = v1.1.0-next", "✓ tag        v1.1.0 moved to HEAD", "next ← main, merged"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}

	if repo.Git("rev-parse", "main~1") != tested {
		t.Fatal("main is not the tested commit plus its release")
	}

	if strings.Contains(repo.Git("log", "--format=%s", "main"), "schedules") {
		t.Fatal("main still holds the untested work")
	}

	if repo.RemoteGit("rev-parse", "v1.1.0^{commit}") != repo.Git("rev-parse", "main") {
		t.Fatal("v1.1.0 was not moved to the tested release on the remote")
	}

	if repo.Git("merge-base", "main", "next") != repo.Git("rev-parse", "main") || repo.RemoteGit("rev-parse", "next") != repo.Git("rev-parse", "next") {
		t.Fatal("next lacks the release, or was not pushed")
	}
}

// TestFlowStableTestedOnly releases the tested tag when next moved on past
// it: the newer commit stays on next, and next gets the release merged in.
func TestFlowStableTestedOnly(t *testing.T) {
	current := newSession(t, STABLE, map[string]string{"package.json": PACKAGE_JSON})
	repo := current.repo

	onNext(current, map[string]string{"package.json": NEXT_PACKAGE})
	repo.Git("tag", "v1.1.0-next")
	repo.Write(map[string]string{"wip.txt": "untested"})
	repo.Commit("wip: not tested yet")
	repo.Git("push", "--quiet", "-u", "origin", "next", "v1.1.0-next")

	flowError := current.app("", true).Dispatch(context.Background(), []string{"final"})
	if flowError != nil {
		t.Fatalf("flow: %v\n%s", flowError, current.output)
	}

	if strings.Contains(repo.Git("log", "--format=%s", "main"), "wip") {
		t.Fatal("main holds the untested commit")
	}

	if !strings.Contains(repo.Git("log", "--format=%s", "next"), "wip") || repo.Git("merge-base", "main", "next") != repo.Git("rev-parse", "main") {
		t.Fatal("next lost its commit, or lacks the release")
	}

	if !strings.Contains(current.output.String(), "next ← main, merged") {
		t.Fatalf("next was not merged:\n%s", current.output)
	}
}
