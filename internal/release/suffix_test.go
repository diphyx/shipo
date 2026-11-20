package release

import (
	"context"
	"testing"

	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/tool/process"
)

const SUFFIX = "[bump]\nsuffix = [\"next\", \"release/*\"]\n[publish]\npush = \"origin/{branch}\"\n"

func planOn(t *testing.T, repo *testkit.Repo, level semver.Level) *Release {
	t.Helper()

	plan, planError := NewRelease(context.Background(), openTarget(t, repo, repo.Work), level)
	if planError != nil {
		t.Fatal(planError)
	}

	return plan
}

func TestSuffixBump(t *testing.T) {
	repo := testkit.NewRepo(t, SUFFIX, map[string]string{"package.json": PACKAGE})
	repo.Git("tag", "v1.2.3")

	repo.Git("checkout", "--quiet", "-b", "next")

	plan := planOn(t, repo, semver.LEVEL_MINOR)
	if plan.Next.String() != "1.3.0-next" || plan.Tag != "v1.3.0-next" || plan.PreviousTag != "v1.2.3" {
		t.Fatalf("next: %s %s %s", plan.Next, plan.Tag, plan.PreviousTag)
	}

	repo.Write(map[string]string{"package.json": `{"name": "app", "version": "1.3.0-next"}`})
	repo.Commit("release next")
	repo.Git("tag", "v1.3.0-next")

	head, headError := FromHead(context.Background(), openTarget(t, repo, repo.Work))
	if headError != nil || head.PreviousTag != "v1.2.3" {
		t.Fatalf("FromHead on next: %v %+v", headError, head)
	}

	patch := planOn(t, repo, semver.LEVEL_PATCH)
	if patch.Next.String() != "1.3.1-next" || patch.PreviousTag != "v1.3.0-next" {
		t.Fatalf("next patch: %s %s", patch.Next, patch.PreviousTag)
	}

	repo.Git("checkout", "--quiet", "-b", "release/2.x")

	group := planOn(t, repo, semver.LEVEL_MAJOR)
	if group.Next.String() != "2.0.0-release-2.x" {
		t.Fatalf("release/2.x: %s", group.Next)
	}

	repo.Git("checkout", "--quiet", "main")
	repo.Git("merge", "--quiet", "next")

	promoted := planOn(t, repo, semver.LEVEL_MINOR)
	if promoted.Next.String() != "1.3.0" || promoted.PreviousTag != "v1.2.3" {
		t.Fatalf("main after merge: %s %s", promoted.Next, promoted.PreviousTag)
	}
}

func TestSuffixGitTags(t *testing.T) {
	repo := testkit.NewRepo(t, "[version]\npreset = \"git\"\n"+SUFFIX, nil)
	repo.Git("tag", "v1.2.0")
	repo.Git("tag", "v1.4.0-next")

	main := planOn(t, repo, semver.LEVEL_PATCH)
	if main.Previous.String() != "1.2.0" || main.Next.String() != "1.2.1" {
		t.Fatalf("main reads %s, next %s", main.Previous, main.Next)
	}

	repo.Git("checkout", "--quiet", "-b", "next")

	next := planOn(t, repo, semver.LEVEL_PATCH)
	if next.Previous.String() != "1.4.0-next" || next.Next.String() != "1.4.1-next" {
		t.Fatalf("next reads %s, next %s", next.Previous, next.Next)
	}
}

func TestSuffixErrors(t *testing.T) {
	repo := testkit.NewRepo(t, "[bump]\nsuffix = \"next\"\n", map[string]string{"package.json": PACKAGE})
	repo.Git("checkout", "--quiet", "--detach")

	target := openTarget(t, repo, repo.Work)

	_, detachedError := NewRelease(context.Background(), target, semver.LEVEL_PATCH)
	if detachedError == nil {
		t.Fatal("detached HEAD: want an error")
	}

	_, currentError := NewRelease(context.Background(), target, semver.LEVEL_CURRENT)
	if currentError != nil {
		t.Fatalf("current needs no branch: %v", currentError)
	}

	odd := testkit.NewRepo(t, "[bump]\nsuffix = \"*\"\n", map[string]string{"package.json": PACKAGE})
	odd.Git("checkout", "--quiet", "-b", "we_ird")

	_, openError := Open(context.Background(), process.System{}, odd.Work)
	if openError == nil {
		t.Fatal("a branch with _ can't be a suffix")
	}
}
