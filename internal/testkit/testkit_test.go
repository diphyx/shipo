package testkit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/tool/process"
)

func TestRepo(t *testing.T) {
	repo := NewRepo(t, "[bump]\nlevel = \"patch\"\n", map[string]string{"dir/a.txt": "a"})

	if repo.Read(".shipo") != "[bump]\nlevel = \"patch\"\n" || repo.Read("dir/a.txt") != "a" {
		t.Fatal("files not written")
	}

	if repo.Git("log", "-1", "--format=%s") != "init" || repo.RemoteGit("rev-parse", "main") != repo.Git("rev-parse", "HEAD") {
		t.Fatal("init not committed and pushed")
	}

	if repo.Git("log", "-1", "--format=%ae") != "test@example.com" {
		t.Fatal("identity not isolated")
	}
}

func TestRunner(t *testing.T) {
	runner := NewRunner("gh")
	runner.Handle = WithReplies(map[string]Response{
		"gh release":      {Output: "release"},
		"gh release view": {Output: "view"},
		"gh fail":         {Error: ErrFailed},
	})

	background := context.Background()

	viewed, _ := runner.Output(background, process.Command{Name: "gh", Arguments: []string{"release", "view", "v1"}})
	listed, _ := runner.Output(background, process.Command{Name: "gh", Arguments: []string{"release", "list"}})
	other, _ := runner.Output(background, process.Command{Name: "gh", Arguments: []string{"auth"}})

	if viewed != "view" || listed != "release" || other != "" {
		t.Fatalf("the longest prefix must win: %q %q %q", viewed, listed, other)
	}

	if runner.Run(background, process.Command{Name: "gh", Arguments: []string{"fail"}}) != ErrFailed {
		t.Fatal("Run must return the fake error")
	}

	real, realError := runner.Output(background, process.Command{Name: "git", Arguments: []string{"--version"}})
	if realError != nil || !strings.HasPrefix(real, "git version") {
		t.Fatal("git must run for real")
	}

	if runner.Run(background, process.Command{Name: "git", Arguments: []string{"--version"}}) != nil || !strings.HasPrefix(runner.Stream.String(), "git version") {
		t.Fatal("real streamed output goes to Stream")
	}

	want := []string{"gh release view v1", "gh release list", "gh auth", "gh fail"}
	if !reflect.DeepEqual(runner.Calls(), want) || !runner.Called("gh release") || runner.Called("git") {
		t.Fatalf("calls = %v", runner.Calls())
	}

	if !runner.Look("gh") || runner.Look("npm") || !runner.Look("git") {
		t.Fatal("Look")
	}

	silent := NewRunner()
	if silent.Run(background, process.Command{Name: "npm"}) != nil {
		t.Fatal("no Handle means success")
	}
}
