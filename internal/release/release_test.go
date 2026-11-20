package release

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/text/keyword"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/tool/process"
)

const PACKAGE = `{"name": "app", "version": "1.2.3"}`

func openTarget(t *testing.T, repo *testkit.Repo, dir string) *Target {
	t.Helper()

	target, openError := Open(context.Background(), process.System{}, dir)
	if openError != nil {
		t.Fatal(openError)
	}

	return target
}

func TestOpen(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE, "web/package.json": `{"version": "0.1.0"}`, "web/.shipo": ""})

	root := openTarget(t, repo, filepath.Join(repo.Work, "web", ".."))
	if root.Folder != "." || root.Version.Preset != "javascript" {
		t.Fatalf("root target: folder %q preset %q", root.Folder, root.Version.Preset)
	}

	web := openTarget(t, repo, filepath.Join(repo.Work, "web"))
	if web.Folder != "web" {
		t.Fatalf("web folder = %q", web.Folder)
	}

	remote, branch, isPushed := web.Push()
	if !isPushed || remote != "origin" || branch != "main" {
		t.Fatalf("Push = %q %q %v", remote, branch, isPushed)
	}
}

func TestOpenErrors(t *testing.T) {
	testkit.Isolate(t)

	_, notFound := Open(context.Background(), process.System{}, t.TempDir())
	if !errors.Is(notFound, config.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", notFound)
	}

	repo := testkit.NewRepo(t, "[bump]\nnope = \"x\"\n", nil)

	_, badConfig := Open(context.Background(), process.System{}, repo.Work)
	if badConfig == nil {
		t.Fatal("want a config error")
	}

	noStack := testkit.NewRepo(t, "", nil)

	_, noPreset := Open(context.Background(), process.System{}, noStack.Work)
	if noPreset == nil {
		t.Fatal("want a no-stack error")
	}
}

func TestPushOff(t *testing.T) {
	repo := testkit.NewRepo(t, "[publish]\npush = false\n", map[string]string{"package.json": PACKAGE})

	_, _, isPushed := openTarget(t, repo, repo.Work).Push()
	if isPushed {
		t.Fatal("push = false must report no push")
	}
}

func TestNewReleaseFromFile(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	repo.Git("tag", "v1.2.3")

	target := openTarget(t, repo, repo.Work)

	release, planError := NewRelease(context.Background(), target, semver.LEVEL_MINOR)
	if planError != nil {
		t.Fatal(planError)
	}

	if release.Previous.String() != "1.2.3" || release.Next.String() != "1.3.0" || release.Tag != "v1.3.0" || release.PreviousTag != "v1.2.3" {
		t.Fatalf("release = %+v", release)
	}

	if len(release.Date) != len("2006-01-02") {
		t.Fatalf("date = %q", release.Date)
	}
}

func TestNewReleaseWithoutPreviousTag(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})

	release, _ := NewRelease(context.Background(), openTarget(t, repo, repo.Work), semver.LEVEL_PATCH)
	if release.PreviousTag != "" {
		t.Fatalf("previous tag must be empty, got %q", release.PreviousTag)
	}
}

func TestNewReleaseFromTags(t *testing.T) {
	repo := testkit.NewRepo(t, "[version]\npreset = \"go\"\n", map[string]string{"go.mod": "module x\n"})
	repo.Git("tag", "v0.9.0")
	repo.Git("tag", "v0.10.0")
	repo.Git("tag", "other")

	release, planError := NewRelease(context.Background(), openTarget(t, repo, repo.Work), semver.LEVEL_PATCH)
	if planError != nil {
		t.Fatal(planError)
	}

	if release.Previous.String() != "0.10.0" || release.Next.String() != "0.10.1" || release.PreviousTag != "v0.10.0" {
		t.Fatalf("release = %+v", release)
	}
}

func TestFromHead(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": `{"version": "1.0.0"}`})
	repo.Git("tag", "v1.0.0")

	repo.Write(map[string]string{"package.json": `{"version": "1.1.0"}`})
	repo.Commit("chore: release v1.1.0")
	repo.Git("tag", "v1.1.0")

	release, headError := FromHead(context.Background(), openTarget(t, repo, repo.Work))
	if headError != nil {
		t.Fatal(headError)
	}

	if release.Tag != "v1.1.0" || release.PreviousTag != "v1.0.0" || release.Previous.String() != "1.0.0" || release.Next.String() != "1.1.0" {
		t.Fatalf("release = %+v", release)
	}

	if release.Level != semver.LEVEL_CURRENT || len(release.Hash) < 7 {
		t.Fatalf("level %q hash %q", release.Level, release.Hash)
	}
}

func TestFromHeadNotARelease(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})

	_, headError := FromHead(context.Background(), openTarget(t, repo, repo.Work))
	if !errors.Is(headError, ErrNoRelease) {
		t.Fatalf("want ErrNoRelease, got %v", headError)
	}
}

func TestFromHeadSubfolderTags(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"web/package.json": `{"version": "2.0.0"}`, "web/.shipo": "", "package.json": PACKAGE})
	repo.Git("tag", "web/v1.0.0")
	repo.Git("tag", "v1.9.0")
	repo.Git("tag", "web/v2.0.0")

	release, headError := FromHead(context.Background(), openTarget(t, repo, filepath.Join(repo.Work, "web")))
	if headError != nil {
		t.Fatal(headError)
	}

	if release.PreviousTag != "web/v1.0.0" {
		t.Fatalf("previous tag = %q; tags of other targets must be ignored", release.PreviousTag)
	}
}

func TestValues(t *testing.T) {
	repo := testkit.NewRepo(t, "[publish]\npush = \"upstream/release/v2\"\n", map[string]string{"package.json": PACKAGE})
	repo.Git("remote", "add", "upstream", repo.Remote)
	repo.Git("tag", "v1.2.3")

	release, _ := NewRelease(context.Background(), openTarget(t, repo, repo.Work), semver.LEVEL_MAJOR)

	values := release.Values()
	want := keyword.Values{
		"version":      "2.0.0",
		"previous":     "1.2.3",
		"major":        "2",
		"minor":        "0",
		"patch":        "0",
		"tag":          "v2.0.0",
		"previous_tag": "v1.2.3",
		"bump":         "major",
		"date":         release.Date,
		"branch":       "release/v2",
		"remote":       "upstream",
		"target":       ".",
	}

	for name, value := range want {
		if values[name] != value {
			t.Fatalf("{%s} = %q, want %q", name, values[name], value)
		}
	}

	_, hasHash := values["hash"]
	if hasHash {
		t.Fatal("{hash} must be unset before the release commit exists")
	}

	release.LoadHash(context.Background())

	if values = release.Values(); len(values["hash"]) < 7 {
		t.Fatalf("{hash} = %q after LoadHash", values["hash"])
	}
}

func TestFromHeadGitTags(t *testing.T) {
	repo := testkit.NewRepo(t, "[version]\npreset = \"go\"\npattern = \"api/v{version}\"\n", map[string]string{"go.mod": "module x\n"})
	repo.Git("tag", "api/v1.0.0")
	repo.Git("tag", "v5.0.0")
	repo.Commit("work")
	repo.Git("tag", "api/v1.1.0")

	release, headError := FromHead(context.Background(), openTarget(t, repo, repo.Work))
	if headError != nil {
		t.Fatal(headError)
	}

	if release.Tag != "api/v1.1.0" || release.PreviousTag != "api/v1.0.0" || release.Previous.String() != "1.0.0" {
		t.Fatalf("release = %+v", release)
	}
}

func TestFromHeadFirstRelease(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{"package.json": PACKAGE})
	repo.Git("tag", "v1.2.3")

	release, headError := FromHead(context.Background(), openTarget(t, repo, repo.Work))
	if headError != nil {
		t.Fatal(headError)
	}

	if release.PreviousTag != "" || release.Previous.String() != "0.0.0" {
		t.Fatalf("the first release has no previous tag: %+v", release)
	}
}
