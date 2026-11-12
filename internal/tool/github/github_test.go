package github

import (
	"context"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/testkit"
)

func TestReleases(t *testing.T) {
	runner := testkit.NewRunner("gh")
	runner.Handle = testkit.WithReplies(map[string]testkit.Response{
		"gh auth status":                        {},
		"gh release view v1.0.0 --json tagName": {Output: `{"tagName":"v1.0.0"}`},
		"gh release view v1.0.0 --json body":    {Output: "kept notes"},
		"gh release view v2.0.0":                {Error: testkit.ErrFailed},
	})

	github := NewGitHub(runner, "/repo")
	background := context.Background()

	if !github.IsLoggedIn(background) {
		t.Fatal("IsLoggedIn")
	}

	if !github.HasRelease(background, "v1.0.0") || github.HasRelease(background, "v2.0.0") {
		t.Fatal("HasRelease")
	}

	notes, notesError := github.ReleaseNotes(background, "v1.0.0")
	if notesError != nil || notes != "kept notes" {
		t.Fatalf("ReleaseNotes = %q, %v", notes, notesError)
	}

	deleteError := github.DeleteRelease(background, "v1.0.0")
	if deleteError != nil || !runner.Called("gh release delete v1.0.0 --yes") {
		t.Fatalf("DeleteRelease: %v %v", deleteError, runner.Calls())
	}
}

func TestLoggedOut(t *testing.T) {
	runner := testkit.NewRunner("gh")
	runner.Handle = testkit.WithReplies(map[string]testkit.Response{"gh auth status": {Error: testkit.ErrFailed}})

	if NewGitHub(runner, "").IsLoggedIn(context.Background()) {
		t.Fatal("must be logged out")
	}
}

func TestCreateRelease(t *testing.T) {
	runner := testkit.NewRunner("gh")
	github := NewGitHub(runner, "/repo")
	background := context.Background()

	generateError := github.CreateRelease(background, nil, Release{Tag: "v1.0.0", Title: "v1.0.0", Files: []string{"dist/a.tgz", "dist/b.tgz"}})
	keepError := github.CreateRelease(background, nil, Release{Tag: "v1.0.0", Title: "v1.0.0", Notes: "kept"})

	if generateError != nil || keepError != nil {
		t.Fatal(generateError, keepError)
	}

	calls := runner.Calls()
	want := []string{
		"gh release create v1.0.0 --verify-tag --title v1.0.0 --generate-notes dist/a.tgz dist/b.tgz",
		"gh release create v1.0.0 --verify-tag --title v1.0.0 --notes kept",
	}

	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s", strings.Join(calls, "\n"))
	}
}

func TestStartWorkflow(t *testing.T) {
	runner := testkit.NewRunner("gh")
	runner.Handle = testkit.WithReplies(map[string]testkit.Response{
		"gh workflow run build.yml": {Output: "✓ Created workflow_dispatch event for build.yml at main\n\nhttps://github.com/org/app/actions/runs/123456\n"},
		"gh workflow run old.yml":   {Output: "✓ Created workflow_dispatch event"},
		"gh workflow run bad.yml":   {Error: testkit.ErrFailed},
	})

	github := NewGitHub(runner, "")
	background := context.Background()

	run, startError := github.StartWorkflow(background, "build.yml", "main", []string{"version=v1.0.0", "title=a b"})
	if startError != nil || run.ID != "123456" || run.URL != "https://github.com/org/app/actions/runs/123456" {
		t.Fatalf("run = %+v, %v", run, startError)
	}

	if !runner.Called("gh workflow run build.yml --ref main --raw-field version=v1.0.0 --raw-field title=a b") {
		t.Fatalf("calls = %v", runner.Calls())
	}

	_, noURLError := github.StartWorkflow(background, "old.yml", "main", nil)
	if noURLError == nil {
		t.Fatal("want an error when gh returns no run URL")
	}

	_, failError := github.StartWorkflow(background, "bad.yml", "main", nil)
	if failError == nil {
		t.Fatal("want the gh error")
	}
}

func TestWatchRun(t *testing.T) {
	runner := testkit.NewRunner("gh")
	runner.Handle = testkit.WithReplies(map[string]testkit.Response{"gh run watch 9": {Error: testkit.ErrFailed}})

	github := NewGitHub(runner, "")

	if github.WatchRun(context.Background(), nil, "1") != nil {
		t.Fatal("a successful run must not fail")
	}

	if github.WatchRun(context.Background(), nil, "9") == nil {
		t.Fatal("a failed run must fail")
	}

	if !runner.Called("gh run watch 1 --exit-status") {
		t.Fatalf("calls = %v", runner.Calls())
	}
}

func TestLatestRun(t *testing.T) {
	runner := testkit.NewRunner("gh")
	runner.Handle = testkit.WithReplies(map[string]testkit.Response{
		"gh run list --workflow done.yml": {Output: `[{"status":"completed","conclusion":"success","url":"https://x/1"}]`},
		"gh run list --workflow none.yml": {Output: `[]`},
		"gh run list --workflow bad.yml":  {Output: `not json`},
		"gh run list --workflow fail.yml": {Error: testkit.ErrFailed},
	})

	github := NewGitHub(runner, "")
	background := context.Background()

	run, isFound, listError := github.LatestRun(background, "done.yml", "abc")
	if listError != nil || !isFound || run.Conclusion != "success" || run.URL != "https://x/1" {
		t.Fatalf("run = %+v %v %v", run, isFound, listError)
	}

	_, isFound, listError = github.LatestRun(background, "none.yml", "abc")
	if listError != nil || isFound {
		t.Fatal("no runs must not be found")
	}

	_, _, badError := github.LatestRun(background, "bad.yml", "abc")
	_, _, failError := github.LatestRun(background, "fail.yml", "abc")

	if badError == nil || failError == nil {
		t.Fatal("want errors for bad JSON and a failed call")
	}

	if !runner.Called("gh run list --workflow done.yml --commit abc --limit 1 --json status,conclusion,url") {
		t.Fatalf("calls = %v", runner.Calls())
	}
}
