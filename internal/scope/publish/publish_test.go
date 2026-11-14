package publish

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/tool/process"
	"github.com/diphyx/shipo/internal/ui"
)

const PACKAGE = `{"version": "1.0.0"}`

type setup struct {
	repo    *testkit.Repo
	runner  *testkit.Runner
	release *release.Release
	output  *bytes.Buffer
	ui      *ui.Terminal
}

// tagged makes a repo whose HEAD is the release v1.1.0, not yet pushed.
func tagged(t *testing.T, shipo string, files map[string]string) *setup {
	t.Helper()

	all := map[string]string{"package.json": PACKAGE}
	for name, content := range files {
		all[name] = content
	}

	repo := testkit.NewRepo(t, shipo, all)
	repo.Write(map[string]string{"package.json": `{"version": "1.1.0"}`})
	repo.Commit("chore: release v1.1.0")
	repo.Git("tag", "--annotate", "v1.1.0", "--message", "Release v1.1.0")

	runner := testkit.NewRunner("gh", "npm")

	target, openError := release.Open(context.Background(), runner, repo.Work)
	if openError != nil {
		t.Fatal(openError)
	}

	head, headError := release.FromHead(context.Background(), target)
	if headError != nil {
		t.Fatal(headError)
	}

	var output bytes.Buffer

	return &setup{repo: repo, runner: runner, release: head, output: &output, ui: ui.NewTestTerminal(&output, "", false)}
}

func TestIsSet(t *testing.T) {
	cases := []struct {
		shipo string
		want  map[string]bool
	}{
		{"", map[string]bool{TASK_PUSH: true}},
		{"[publish]\nrelease = true\nworkflow = \"b.yml\"\ncommand = \"x\"\n", map[string]bool{TASK_PUSH: true, TASK_RELEASE: true, TASK_WORKFLOW: true, TASK_COMMAND: true}},
		{"[publish]\npush = false\nrelease = true\nworkflow = \"b.yml\"\ncommand = \"x\"\n", map[string]bool{TASK_COMMAND: true}},
		{"[publish]\nrelease = false\nworkflow = false\n", map[string]bool{TASK_PUSH: true}},
	}

	for _, testCase := range cases {
		current := tagged(t, testCase.shipo, nil)

		for _, task := range append(Tasks, "unknown") {
			if IsSet(current.release.Target, task) != testCase.want[task] {
				t.Fatalf("%q: IsSet(%s) = %v", testCase.shipo, task, !testCase.want[task])
			}
		}
	}
}

func TestPrograms(t *testing.T) {
	current := tagged(t, "[publish]\ncommand = [\"npm publish\", \"./deploy.sh\", \"echo 'x\"]\n", nil)
	target := current.release.Target

	if !reflect.DeepEqual(Programs(target, TASK_PUSH), []string{"git"}) || !reflect.DeepEqual(Programs(target, TASK_RELEASE), []string{"gh"}) {
		t.Fatal("push needs git, release needs gh")
	}

	if !reflect.DeepEqual(Programs(target, TASK_COMMAND), []string{"npm"}) {
		t.Fatalf("command programs = %v; paths and bad lines are skipped", Programs(target, TASK_COMMAND))
	}

	if Programs(target, "unknown") != nil {
		t.Fatal("unknown task")
	}
}

func TestDescribe(t *testing.T) {
	shipo := "[publish]\npush = [\"origin/main\", \"mirror/main\"]\nrelease = [\"dist/*\", \"notes.md\"]\nworkflow = \"build.yml version={tag}\"\ncommand = \"deploy {version}\"\n"
	current := tagged(t, shipo, nil)

	want := map[string][]string{
		TASK_PUSH:     {"origin/main + v1.1.0", "mirror/main + v1.1.0"},
		TASK_RELEASE:  {"v1.1.0 + dist/*, notes.md"},
		TASK_WORKFLOW: {"build.yml version=v1.1.0"},
		TASK_COMMAND:  {"deploy 1.1.0"},
	}

	for task, lines := range want {
		if got := Describe(current.release, task); !reflect.DeepEqual(got, lines) {
			t.Fatalf("Describe(%s) = %q, want %q", task, got, lines)
		}
	}

	onlyTag := tagged(t, "[publish]\nrelease = true\n", nil)
	if got := Describe(onlyTag.release, TASK_RELEASE); !reflect.DeepEqual(got, []string{"v1.1.0"}) {
		t.Fatalf("release = true: %q", got)
	}
}

func TestPush(t *testing.T) {
	current := tagged(t, "", nil)

	pushError := Push(context.Background(), current.release, current.ui)
	if pushError != nil {
		t.Fatal(pushError)
	}

	if current.repo.RemoteGit("tag") != "v1.1.0" || current.repo.RemoteGit("rev-parse", "main") != current.repo.Git("rev-parse", "HEAD") {
		t.Fatal("the release commit and tag must be on the remote")
	}

	if !strings.Contains(current.output.String(), "✓ push       origin/main + v1.1.0") {
		t.Fatalf("output:\n%s", current.output)
	}
}

func TestPushFromFeatureBranch(t *testing.T) {
	current := tagged(t, "", nil)
	repo := current.repo

	repo.Git("tag", "--delete", "v1.1.0")
	repo.Git("reset", "--quiet", "--hard", "HEAD~1")
	repo.Git("checkout", "--quiet", "-b", "feature")
	repo.Write(map[string]string{"package.json": `{"version": "1.1.0"}`})
	repo.Commit("chore: release v1.1.0")
	repo.Git("tag", "v1.1.0")

	target, _ := release.Open(context.Background(), current.runner, repo.Work)
	head, _ := release.FromHead(context.Background(), target)

	pushError := Push(context.Background(), head, current.ui)
	if pushError != nil {
		t.Fatal(pushError)
	}

	commit := repo.Git("rev-parse", "HEAD")

	if repo.RemoteGit("rev-parse", "main") != commit || repo.RemoteGit("rev-parse", "feature") != commit {
		t.Fatal("main and the feature branch must both be pushed")
	}

	if repo.Git("rev-parse", "main") != commit {
		t.Fatal("the local main must be fast-forwarded")
	}
}

func TestPushToSeveralRemotes(t *testing.T) {
	current := tagged(t, "[publish]\npush = [\"origin/main\", \"mirror/release\"]\n", nil)
	current.repo.Git("remote", "add", "mirror", current.repo.Remote)

	pushError := Push(context.Background(), current.release, current.ui)
	if pushError != nil {
		t.Fatal(pushError)
	}

	if current.repo.RemoteGit("rev-parse", "release") != current.repo.Git("rev-parse", "HEAD") {
		t.Fatal("the second entry must be pushed too")
	}
}

func TestPushRejected(t *testing.T) {
	current := tagged(t, "[publish]\npush = \"nope/main\"\n", nil)

	pushError := Push(context.Background(), current.release, current.ui)
	if pushError == nil || !strings.Contains(pushError.Error(), "push to nope/main") {
		t.Fatalf("want a push error, got %v", pushError)
	}
}

func TestReleaseFiles(t *testing.T) {
	files := map[string]string{"dist/a.tgz": "a", "dist/b.tgz": "b", "dist/sub/c.txt": "c", "notes.md": "n"}

	cases := []struct {
		release string
		want    []string
		fail    bool
	}{
		{"[publish]\nrelease = \"dist/\"\n", []string{"dist/a.tgz", "dist/b.tgz"}, false},
		{"[publish]\nrelease = \"dist\"\n", []string{"dist/a.tgz", "dist/b.tgz"}, false},
		{"[publish]\nrelease = [\"dist/*.tgz\", \"notes.md\"]\n", []string{"dist/a.tgz", "dist/b.tgz", "notes.md"}, false},
		{"[publish]\nrelease = true\n", nil, false},
		{"[publish]\nrelease = \"missing/*\"\n", nil, true},
		{"[publish]\nrelease = \"dist/[\"\n", nil, true},
	}

	for _, testCase := range cases {
		current := tagged(t, testCase.release, files)

		got, filesError := ReleaseFiles(current.release.Target)
		if testCase.fail != (filesError != nil) {
			t.Fatalf("%q: error = %v", testCase.release, filesError)
		}

		relative := make([]string, len(got))
		for index, file := range got {
			relative[index] = filepath.ToSlash(current.release.Config.Rel(file))
		}

		if !testCase.fail && len(got) > 0 && !reflect.DeepEqual(relative, testCase.want) {
			t.Fatalf("%q: files = %v, want %v", testCase.release, relative, testCase.want)
		}
	}
}

func TestRelease(t *testing.T) {
	current := tagged(t, "[publish]\nrelease = \"notes.md\"\n", map[string]string{"notes.md": "n"})
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{"gh release view": {Error: testkit.ErrFailed}})

	releaseError := Release(context.Background(), current.release, current.ui)
	if releaseError != nil {
		t.Fatal(releaseError)
	}

	created := current.runner.Calls()[len(current.runner.Calls())-1]
	if !strings.HasPrefix(created, "gh release create v1.1.0 --verify-tag --title v1.1.0 --generate-notes ") || !strings.HasSuffix(created, "notes.md") {
		t.Fatalf("create = %q", created)
	}

	if !strings.Contains(current.output.String(), "✓ release    v1.1.0 + 1 file(s)") {
		t.Fatalf("output:\n%s", current.output)
	}
}

func TestReleaseExists(t *testing.T) {
	current := tagged(t, "[publish]\nrelease = true\n", nil)
	current.release.Level = semver.LEVEL_MINOR

	releaseError := Release(context.Background(), current.release, current.ui)
	if releaseError == nil || !strings.Contains(releaseError.Error(), "already exists") {
		t.Fatalf("want already exists, got %v", releaseError)
	}
}

func TestReleaseCurrentKeepsNotes(t *testing.T) {
	current := tagged(t, "[publish]\nrelease = true\n", nil)
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{"gh release view v1.1.0 --json body": {Output: "hand-written notes"}})

	releaseError := Release(context.Background(), current.release, current.ui)
	if releaseError != nil {
		t.Fatal(releaseError)
	}

	if !current.runner.Called("gh release delete v1.1.0 --yes") || !current.runner.Called("gh release create v1.1.0 --verify-tag --title v1.1.0 --notes hand-written notes") {
		t.Fatalf("calls:\n%s", strings.Join(current.runner.Calls(), "\n"))
	}
}

func TestReleaseNeedsPush(t *testing.T) {
	current := tagged(t, "[publish]\npush = false\nrelease = true\n", nil)

	if !errors.Is(Release(context.Background(), current.release, current.ui), ErrNeedsPush) {
		t.Fatal("release needs push")
	}

	if !errors.Is(Workflow(context.Background(), current.release, current.ui, false), ErrNeedsPush) {
		t.Fatal("workflow needs push")
	}
}

func TestWorkflow(t *testing.T) {
	current := tagged(t, "[publish]\nworkflow = [\"build.yml version={tag}\", \"docs.yml\"]\n", nil)
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{
		"gh workflow run build.yml": {Output: "https://github.com/o/r/actions/runs/1"},
		"gh workflow run docs.yml":  {Output: "https://github.com/o/r/actions/runs/2"},
	})

	workflowError := Workflow(context.Background(), current.release, current.ui, false)
	if workflowError != nil {
		t.Fatal(workflowError)
	}

	want := []string{
		"gh workflow run build.yml --ref main --raw-field version=v1.1.0",
		"gh run watch 1 --exit-status",
		"gh workflow run docs.yml --ref main",
		"gh run watch 2 --exit-status",
	}

	if !reflect.DeepEqual(current.runner.Calls(), want) {
		t.Fatalf("calls:\n%s", strings.Join(current.runner.Calls(), "\n"))
	}
}

func TestWorkflowNoWait(t *testing.T) {
	current := tagged(t, "[publish]\nworkflow = \"build.yml\"\n", nil)
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{"gh workflow run": {Output: "https://github.com/o/r/actions/runs/7"}})

	workflowError := Workflow(context.Background(), current.release, current.ui, true)
	if workflowError != nil {
		t.Fatal(workflowError)
	}

	if current.runner.Called("gh run watch") || !strings.Contains(current.output.String(), "build.yml started https://github.com/o/r/actions/runs/7") {
		t.Fatalf("calls = %v\n%s", current.runner.Calls(), current.output)
	}
}

func TestWorkflowFailures(t *testing.T) {
	current := tagged(t, "[publish]\nworkflow = [\"build.yml\", \"docs.yml\"]\n", nil)
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{
		"gh workflow run": {Output: "https://github.com/o/r/actions/runs/5"},
		"gh run watch 5":  {Error: testkit.ErrFailed},
	})

	workflowError := Workflow(context.Background(), current.release, current.ui, false)
	if workflowError == nil || !strings.Contains(workflowError.Error(), "build.yml did not succeed") {
		t.Fatalf("want a failed run, got %v", workflowError)
	}

	if current.runner.Called("gh workflow run docs.yml") {
		t.Fatal("a failed run must stop the next workflow")
	}

	startFail := tagged(t, "[publish]\nworkflow = \"build.yml\"\n", nil)
	startFail.runner.Handle = testkit.WithReplies(map[string]testkit.Response{"gh workflow run": {Error: testkit.ErrFailed}})

	if Workflow(context.Background(), startFail.release, startFail.ui, false) == nil {
		t.Fatal("want a start error")
	}

	badInput := tagged(t, "[publish]\nworkflow = \"build.yml noequals\"\n", nil)
	if Workflow(context.Background(), badInput.release, badInput.ui, false) == nil {
		t.Fatal("want an input error")
	}
}

func TestWorkflowStopWatching(t *testing.T) {
	current := tagged(t, "[publish]\nworkflow = [\"build.yml\", \"docs.yml\"]\n", nil)
	stopped, stop := context.WithCancel(context.Background())
	replies := testkit.WithReplies(map[string]testkit.Response{
		"gh workflow run": {Output: "https://github.com/o/r/actions/runs/5"},
		"gh run watch 5":  {Error: testkit.ErrFailed},
	})
	current.runner.Handle = func(command process.Command) testkit.Response {
		if strings.HasPrefix(command.String(), "gh run watch") {
			stop()
		}

		return replies(command)
	}

	workflowError := Workflow(stopped, current.release, current.ui, false)
	if !errors.Is(workflowError, ErrStoppedWatching) {
		t.Fatalf("Ctrl-C while watching must only stop the wait, got %v", workflowError)
	}

	if !strings.Contains(current.output.String(), "build.yml still running https://github.com/o/r/actions/runs/5") || current.runner.Called("gh workflow run docs.yml") {
		t.Fatalf("calls = %v\n%s", current.runner.Calls(), current.output)
	}
}

func TestParseWorkflow(t *testing.T) {
	file, inputs, parseError := ParseWorkflow(`build.yml version=v1.2.0 title="a b"`)
	if parseError != nil {
		t.Fatal(parseError)
	}

	if file != "build.yml" || !reflect.DeepEqual(inputs, []string{"version=v1.2.0", "title=a b"}) {
		t.Fatalf("got %q %q", file, inputs)
	}

	for _, entry := range []string{"build.yml noequals", "build.yml =x", "build.yml 'open"} {
		_, _, badError := ParseWorkflow(entry)
		if badError == nil {
			t.Fatalf("want an error for %q", entry)
		}
	}
}

func TestCommand(t *testing.T) {
	current := tagged(t, "[publish]\ncommand = [\"npm publish --tag {major}.{minor}\", \"npm view app@{version}\"]\n", nil)

	commandError := Command(context.Background(), current.release, current.ui)
	if commandError != nil {
		t.Fatal(commandError)
	}

	if !reflect.DeepEqual(current.runner.Calls(), []string{"npm publish --tag 1.1", "npm view app@1.1.0"}) {
		t.Fatalf("calls = %v", current.runner.Calls())
	}
}

func TestCommandStopsOnFailure(t *testing.T) {
	current := tagged(t, "[publish]\ncommand = [\"npm publish\", \"npm view app\"]\n", nil)
	current.runner.Handle = testkit.WithReplies(map[string]testkit.Response{"npm publish": {Error: testkit.ErrFailed}})

	if Command(context.Background(), current.release, current.ui) == nil || current.runner.Called("npm view") {
		t.Fatal("the first failure must stop the rest")
	}

	badQuote := tagged(t, "[publish]\ncommand = \"echo 'x\"\n", nil)
	if Command(context.Background(), badQuote.release, badQuote.ui) == nil {
		t.Fatal("want a parse error")
	}
}

func TestPushCurrentReplacesRemoteTag(t *testing.T) {
	current := tagged(t, "", nil)
	repo := current.repo

	firstError := Push(context.Background(), current.release, current.ui)
	if firstError != nil {
		t.Fatal(firstError)
	}

	repo.Git("tag", "--delete", "v1.1.0")
	repo.Write(map[string]string{"fix.txt": "fixed\n"})
	repo.Commit("fix: the release")
	repo.Git("tag", "--annotate", "v1.1.0", "--message", "Release v1.1.0")

	target, _ := release.Open(context.Background(), current.runner, repo.Work)
	again, _ := release.FromHead(context.Background(), target)

	pushError := Push(context.Background(), again, current.ui)
	if pushError != nil {
		t.Fatal(pushError)
	}

	if repo.RemoteGit("rev-parse", "v1.1.0^{commit}") != repo.Git("rev-parse", "HEAD") {
		t.Fatal("current must replace the remote tag with the one on HEAD")
	}
}
