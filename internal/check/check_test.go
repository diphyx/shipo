package check

import (
	"context"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/text/semver"
)

const PACKAGE = `{"version": "1.0.0"}`

func planned(t *testing.T, repo *testkit.Repo, runner *testkit.Runner, level semver.Level) *release.Release {
	t.Helper()

	target, openError := release.Open(context.Background(), runner, repo.Work)
	if openError != nil {
		t.Fatal(openError)
	}

	next, planError := release.NewRelease(context.Background(), target, level)
	if planError != nil {
		t.Fatal(planError)
	}

	return next
}

func item(items []Item, name string) (Item, bool) {
	for _, found := range items {
		if found.Name == name {
			return found, true
		}
	}

	return Item{}, false
}

func failure(t *testing.T, items []Item, name string, detail string) {
	t.Helper()

	for _, found := range items {
		if found.Name == name && !found.IsOK && strings.Contains(found.Detail, detail) {
			return
		}
	}

	t.Fatalf("want a failed %s check with %q, got %+v", name, detail, items)
}

func TestAllGood(t *testing.T) {
	repo := testkit.NewRepo(t, "[build]\ncommand = \"git --version\"\n", map[string]string{"package.json": PACKAGE})

	items := Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Everything())
	if Error(items) != nil || len(Failed(items)) != 0 {
		t.Fatalf("want all checks to pass, got %+v", items)
	}

	tools, _ := item(items, "tools")
	remote, _ := item(items, "remote")

	if tools.Detail != "git" || remote.Detail != "origin/main, fast-forward ok" {
		t.Fatalf("tools %q, remote %q", tools.Detail, remote.Detail)
	}
}

func TestTools(t *testing.T) {
	repo := testkit.NewRepo(t, "[publish]\nrelease = true\ncommand = \"npm publish\"\n", map[string]string{"package.json": PACKAGE, "pnpm-lock.yaml": ""})

	missing := Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Everything())
	failure(t, missing, "tools", "not installed: gh, npm, pnpm")

	loggedOut := testkit.NewRunner("gh", "npm", "pnpm")
	loggedOut.Handle = testkit.WithReplies(map[string]testkit.Response{"gh auth status": {Error: testkit.ErrFailed}})

	failure(t, Run(context.Background(), planned(t, repo, loggedOut, semver.LEVEL_PATCH), Everything()), "tools", "gh is not logged in")

	ready := testkit.NewRunner("gh", "npm", "pnpm")

	tools, _ := item(Run(context.Background(), planned(t, repo, ready, semver.LEVEL_PATCH), Everything()), "tools")
	if !tools.IsOK || tools.Detail != "gh, git, npm, pnpm" {
		t.Fatalf("tools = %+v", tools)
	}
}

func TestToolsOnlyForChosenTasks(t *testing.T) {
	repo := testkit.NewRepo(t, "[publish]\nrelease = true\ncommand = \"npm publish\"\n", map[string]string{"package.json": PACKAGE})

	tools, _ := item(Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Options{Tasks: []string{"push"}}), "tools")
	if !tools.IsOK {
		t.Fatalf("only the chosen tasks' tools are needed, got %+v", tools)
	}
}

func TestTree(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	repo.Write(map[string]string{"new.txt": "x"})

	failure(t, Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Everything()), "tree", "has changes")

	_, hasTree := item(Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Options{}), "tree")
	if hasTree {
		t.Fatal("the tree is only checked for a bump")
	}
}

func TestRemotes(t *testing.T) {
	unknown := testkit.NewRepo(t, "[publish]\npush = \"nope/main\"\n", map[string]string{"package.json": PACKAGE})
	failure(t, Run(context.Background(), planned(t, unknown, testkit.NewRunner(), semver.LEVEL_PATCH), Everything()), "remote", "nope is not a configured remote")

	broken := testkit.NewRepo(t, "[publish]\npush = \"dead/main\"\n", map[string]string{"package.json": PACKAGE})
	broken.Git("remote", "add", "dead", "/no/such/remote.git")
	failure(t, Run(context.Background(), planned(t, broken, testkit.NewRunner(), semver.LEVEL_PATCH), Everything()), "remote", "git fetch dead failed")

	fresh := testkit.NewRepo(t, "[publish]\npush = \"origin/release\"\n", map[string]string{"package.json": PACKAGE})

	remote, _ := item(Run(context.Background(), planned(t, fresh, testkit.NewRunner(), semver.LEVEL_PATCH), Everything()), "remote")
	if !remote.IsOK || remote.Detail != "origin/release, new branch" {
		t.Fatalf("remote = %+v", remote)
	}
}

func TestRemoteAhead(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})

	other := t.TempDir()
	testkit.Run(t, other, "git", "clone", "--quiet", repo.Remote, "clone")
	testkit.Run(t, other+"/clone", "git", "commit", "--quiet", "--allow-empty", "-m", "someone else")
	testkit.Run(t, other+"/clone", "git", "push", "--quiet", "origin", "main")

	failure(t, Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Everything()), "remote", "has commits you don't have")
}

func TestTagTaken(t *testing.T) {
	local := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	local.Git("tag", "v1.0.1")
	failure(t, Run(context.Background(), planned(t, local, testkit.NewRunner(), semver.LEVEL_PATCH), Everything()), "tag", "v1.0.1 already exists")

	remote := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	remote.Git("tag", "v1.0.1")
	remote.Git("push", "--quiet", "origin", "v1.0.1")
	remote.Git("tag", "--delete", "v1.0.1")
	failure(t, Run(context.Background(), planned(t, remote, testkit.NewRunner(), semver.LEVEL_PATCH), Everything()), "tag", "v1.0.1 already exists on origin")

	current := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	current.Git("tag", "v1.0.0")

	if Error(Run(context.Background(), planned(t, current, testkit.NewRunner(), semver.LEVEL_CURRENT), Everything())) != nil {
		t.Fatal("current reuses its tag, so an existing tag is fine")
	}
}

func TestBuildAndWorkflowChecks(t *testing.T) {
	repo := testkit.NewRepo(t, "[build]\ndir = \"nope\"\ncommand = \"git --version\"\n\n[publish]\nworkflow = \"build.yml noequals\"\n", map[string]string{"package.json": PACKAGE})
	items := Run(context.Background(), planned(t, repo, testkit.NewRunner("gh"), semver.LEVEL_PATCH), Everything())

	failure(t, items, "build", "does not exist")
	failure(t, items, "workflow", "is not name=value")
}

func TestError(t *testing.T) {
	if Error([]Item{{Name: "a", IsOK: true}}) != nil {
		t.Fatal("all OK must give no error")
	}

	checksError := Error([]Item{{Name: "tree", Detail: "dirty"}, {Name: "tools", IsOK: true}, {Name: "tag", Detail: "taken"}})
	if checksError == nil || checksError.Error() != "checks failed:\n  tree: dirty\n  tag: taken" {
		t.Fatalf("error = %v", checksError)
	}
}

func headRelease(t *testing.T, repo *testkit.Repo, runner *testkit.Runner) *release.Release {
	t.Helper()

	target, _ := release.Open(context.Background(), runner, repo.Work)

	head, headError := release.FromHead(context.Background(), target)
	if headError != nil {
		t.Fatal(headError)
	}

	return head
}

func TestStateOfTag(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	repo.Git("tag", "v1.0.0")

	tag, _ := item(State(context.Background(), headRelease(t, repo, testkit.NewRunner())), "tag")
	if tag.IsOK || tag.Detail != "local, not pushed" {
		t.Fatalf("tag = %+v", tag)
	}

	repo.Git("push", "--quiet", "origin", "v1.0.0")

	tag, _ = item(State(context.Background(), headRelease(t, repo, testkit.NewRunner())), "tag")
	if !tag.IsOK || tag.Detail != "local, remote" {
		t.Fatalf("tag = %+v", tag)
	}

	local := testkit.NewRepo(t, "[publish]\npush = false\n", map[string]string{"package.json": PACKAGE})
	local.Git("tag", "v1.0.0")

	tag, _ = item(State(context.Background(), headRelease(t, local, testkit.NewRunner())), "tag")
	if !tag.IsOK || tag.Detail != "local" {
		t.Fatalf("with push off, a local tag is complete: %+v", tag)
	}
}

func TestStateOfReleaseAndWorkflows(t *testing.T) {
	repo := testkit.NewRepo(t, "[publish]\nrelease = true\nworkflow = [\"done.yml\", \"red.yml\", \"busy.yml\", \"never.yml\", \"bad noequals\"]\n", map[string]string{"package.json": PACKAGE})
	repo.Git("tag", "v1.0.0")

	runner := testkit.NewRunner("gh")
	runner.Handle = testkit.WithReplies(map[string]testkit.Response{
		"gh release view":                  {Error: testkit.ErrFailed},
		"gh run list --workflow done.yml":  {Output: `[{"status":"completed","conclusion":"success"}]`},
		"gh run list --workflow red.yml":   {Output: `[{"status":"completed","conclusion":"failure"}]`},
		"gh run list --workflow busy.yml":  {Output: `[{"status":"in_progress","conclusion":""}]`},
		"gh run list --workflow never.yml": {Output: `[]`},
	})

	items := State(context.Background(), headRelease(t, repo, runner))

	want := []Item{
		{Name: "tag", Detail: "local, not pushed"},
		{Name: "release", Detail: "not created"},
		{Name: "workflow", IsOK: true, Detail: "done.yml success"},
		{Name: "workflow", Detail: "red.yml failure"},
		{Name: "workflow", Detail: "busy.yml in_progress"},
		{Name: "workflow", Detail: "never.yml not run"},
	}

	if len(items) != len(want) {
		t.Fatalf("items = %+v", items)
	}

	for index := range want {
		if items[index] != want[index] {
			t.Fatalf("item %d = %+v, want %+v", index, items[index], want[index])
		}
	}
}

func TestStateWithoutGh(t *testing.T) {
	repo := testkit.NewRepo(t, "[publish]\nrelease = true\nworkflow = \"build.yml\"\n", map[string]string{"package.json": PACKAGE})
	repo.Git("tag", "v1.0.0")

	items := State(context.Background(), headRelease(t, repo, testkit.NewRunner()))
	if len(items) != 1 || items[0].Name != "tag" {
		t.Fatalf("without gh only the tag is known, got %+v", items)
	}
}

func TestTwoLockFiles(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE, "yarn.lock": "", "package-lock.json": "{}"})

	failure(t, Run(context.Background(), planned(t, repo, testkit.NewRunner("npm", "yarn"), semver.LEVEL_PATCH), Everything()), "lock", "several lock files")
}

func TestWorkflowFiles(t *testing.T) {
	files := map[string]string{
		"package.json":                PACKAGE,
		".github/workflows/ok.yml":    "on:\n  workflow_dispatch:\n    inputs: {}\n",
		".github/workflows/push.yaml": "on:\n  push:\n",
	}

	shipo := "[publish]\nworkflow = [\"ok.yml\", \"push.yaml\", \"missing.yml\", \"'Release by name'\"]\n"
	repo := testkit.NewRepo(t, shipo, files)

	items := Run(context.Background(), planned(t, repo, testkit.NewRunner("gh"), semver.LEVEL_PATCH), Everything())

	failure(t, items, "workflow", "push.yaml has no on: workflow_dispatch")
	failure(t, items, "workflow", "missing.yml not found in .github/workflows")

	for _, found := range items {
		if found.Name == "workflow" && (strings.Contains(found.Detail, "ok.yml") || strings.Contains(found.Detail, "Release")) {
			t.Fatalf("a dispatchable file, or a workflow given by name, must pass: %+v", found)
		}
	}
}

func TestFetchIgnoresADifferentLocalTag(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	repo.Git("tag", "--annotate", "v1.0.0", "--message", "first")
	repo.Git("push", "--quiet", "origin", "v1.0.0")
	repo.Git("tag", "--delete", "v1.0.0")
	repo.Git("tag", "--annotate", "v1.0.0", "--message", "again")

	items := Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_CURRENT), Everything())

	remote, _ := item(items, "remote")
	if !remote.IsOK {
		t.Fatalf("a local tag that differs from the remote's must not fail the fetch: %+v", items)
	}
}

func TestFetchFailureSaysWhy(t *testing.T) {
	repo := testkit.NewRepo(t, "[publish]\npush = \"dead/main\"\n", map[string]string{"package.json": PACKAGE})
	repo.Git("remote", "add", "dead", "/no/such/remote.git")

	failure(t, Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Everything()), "remote", "git fetch dead failed: ")
}
