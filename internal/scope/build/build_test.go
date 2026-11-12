package build

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/ui"
)

func plan(t *testing.T, shipo string, files map[string]string, runner *testkit.Runner) (*testkit.Repo, *release.Release) {
	t.Helper()

	all := map[string]string{"package.json": `{"version": "1.0.0"}`}
	for name, content := range files {
		all[name] = content
	}

	repo := testkit.NewRepo(t, shipo, all)

	target, openError := release.Open(context.Background(), runner, repo.Work)
	if openError != nil {
		t.Fatal(openError)
	}

	planned, planError := release.NewRelease(context.Background(), target, semver.LEVEL_MINOR)
	if planError != nil {
		t.Fatal(planError)
	}

	return repo, planned
}

func TestIsSetAndDir(t *testing.T) {
	_, without := plan(t, "", nil, testkit.NewRunner())
	if IsSet(without.Target) || Check(without.Target) != nil {
		t.Fatal("no [build] must be unset and pass the check")
	}

	repo, with := plan(t, "[build]\ndir = \"web\"\ncommand = \"make\"\n", map[string]string{"web/x": ""}, testkit.NewRunner())

	resolvedDir, _ := filepath.EvalSymlinks(Dir(with.Target))
	resolvedWant, _ := filepath.EvalSymlinks(filepath.Join(repo.Work, "web"))

	if !IsSet(with.Target) || resolvedDir != resolvedWant {
		t.Fatalf("Dir = %s", Dir(with.Target))
	}
}

func TestCommands(t *testing.T) {
	shipo := "[build]\ncommand = [\"docker build -t app:{version} .\", \"go build -ldflags '-X main.v={tag}'\"]\nenvironment = [\"APP_VERSION={version}\", \"FROM={previous}\"]\n"
	_, planned := plan(t, shipo, nil, testkit.NewRunner())

	commands, commandsError := Commands(planned)
	if commandsError != nil {
		t.Fatal(commandsError)
	}

	if len(commands) != 2 {
		t.Fatalf("commands = %+v", commands)
	}

	if commands[0].Name != "docker" || !reflect.DeepEqual(commands[0].Arguments, []string{"build", "-t", "app:1.1.0", "."}) {
		t.Fatalf("first = %+v", commands[0])
	}

	if !reflect.DeepEqual(commands[1].Arguments, []string{"build", "-ldflags", "-X main.v=v1.1.0"}) {
		t.Fatalf("second = %+v", commands[1])
	}

	if !reflect.DeepEqual(commands[0].Environment, []string{"APP_VERSION=1.1.0", "FROM=1.0.0"}) || commands[0].Dir != Dir(planned.Target) {
		t.Fatalf("environment = %v, dir = %s", commands[0].Environment, commands[0].Dir)
	}
}

func TestCommandsParseError(t *testing.T) {
	_, planned := plan(t, "[build]\ncommand = \"echo 'open\"\n", nil, testkit.NewRunner())

	_, commandsError := Commands(planned)
	if commandsError == nil {
		t.Fatal("want an unclosed quote error")
	}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name  string
		shipo string
		files map[string]string
		fail  string
	}{
		{"missing dir", "[build]\ndir = \"nope\"\ncommand = \"git --version\"\n", nil, "does not exist"},
		{"dir is a file", "[build]\ndir = \"file\"\ncommand = \"git --version\"\n", map[string]string{"file": "x"}, "does not exist"},
		{"missing program", "[build]\ncommand = \"shipo-no-such-tool\"\n", nil, "is not installed"},
		{"bad quote", "[build]\ncommand = \"echo 'x\"\n", nil, "unclosed"},
		{"script path", "[build]\ncommand = \"./scripts/build.sh\"\n", nil, ""},
		{"installed", "[build]\ncommand = [\"git --version\", \"git status\"]\n", nil, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, planned := plan(t, testCase.shipo, testCase.files, testkit.NewRunner())

			checkError := Check(planned.Target)

			if testCase.fail == "" && checkError != nil {
				t.Fatalf("want no error, got %v", checkError)
			}

			if testCase.fail != "" && (checkError == nil || !strings.Contains(checkError.Error(), testCase.fail)) {
				t.Fatalf("want %q, got %v", testCase.fail, checkError)
			}
		})
	}
}

func TestRun(t *testing.T) {
	runner := testkit.NewRunner("make")
	_, planned := plan(t, "[build]\ncommand = [\"git --version\", \"make dist VERSION={version}\"]\n", nil, runner)

	var output bytes.Buffer

	runError := Run(context.Background(), planned, ui.NewTestTerminal(&output, "", false))
	if runError != nil {
		t.Fatal(runError)
	}

	if !runner.Called("make dist VERSION=1.1.0") {
		t.Fatalf("calls = %v", runner.Calls())
	}

	if !strings.Contains(output.String(), "✓ build      [1/2] git --version") || !strings.Contains(output.String(), "✓ build      [2/2] make dist VERSION=1.1.0") {
		t.Fatalf("output:\n%s", output.String())
	}
}

func TestRunStopsOnFailure(t *testing.T) {
	runner := testkit.NewRunner("make", "npm")
	runner.Handle = testkit.WithReplies(map[string]testkit.Response{"make": {Error: testkit.ErrFailed}})

	_, planned := plan(t, "[build]\ncommand = [\"make\", \"npm run build\"]\n", nil, runner)

	runError := Run(context.Background(), planned, ui.NewTestTerminal(&bytes.Buffer{}, "", false))
	if runError == nil {
		t.Fatal("want the make error")
	}

	if runner.Called("npm") {
		t.Fatal("the first failure must stop the rest")
	}
}

func TestRunRejectsTrackedChanges(t *testing.T) {
	_, planned := plan(t, "[build]\ncommand = \"git rm --quiet --cached package.json\"\n", nil, testkit.NewRunner())

	runError := Run(context.Background(), planned, ui.NewTestTerminal(&bytes.Buffer{}, "", false))
	if runError == nil || !strings.Contains(runError.Error(), "changed tracked files") {
		t.Fatalf("want a tracked-changes error, got %v", runError)
	}
}

func TestRunAllowsUntrackedOutput(t *testing.T) {
	_, planned := plan(t, "[build]\ncommand = \"git init --quiet dist\"\n", nil, testkit.NewRunner())

	runError := Run(context.Background(), planned, ui.NewTestTerminal(&bytes.Buffer{}, "", false))
	if runError != nil {
		t.Fatalf("new untracked files must be fine, got %v", runError)
	}
}
