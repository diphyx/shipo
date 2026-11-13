package release

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/tool/process"
)

func TestNewTargets(t *testing.T) {
	repo := testkit.NewRepo(t, "", map[string]string{
		"web/.shipo":              "",
		"api/v2/.shipo":           "",
		"node_modules/dep/.shipo": "",
		".gitignore":              "node_modules/\n",
	})
	repo.Write(map[string]string{"new/.shipo": "", "removed/.shipo": ""})
	repo.Commit("more")
	repo.Write(map[string]string{"draft/.shipo": ""})

	removeError := os.Remove(filepath.Join(repo.Work, "removed", ".shipo"))
	if removeError != nil {
		t.Fatal(removeError)
	}

	targets, discoverError := NewTargets(context.Background(), process.System{}, filepath.Join(repo.Work, "web"))
	if discoverError != nil {
		t.Fatal(discoverError)
	}

	want := []string{".", "api/v2", "draft", "new", "web"}
	if !reflect.DeepEqual(targets.Folders, want) {
		t.Fatalf("folders = %v; untracked ones count, ignored and deleted ones don't", targets.Folders)
	}

	if nearest := targets.Nearest(filepath.Join(repo.Work, "api", "v2", "src")); nearest != "api/v2" {
		t.Fatalf("nearest = %q", nearest)
	}

	if nearest := targets.Nearest(repo.Work); nearest != "." {
		t.Fatalf("nearest at the root = %q", nearest)
	}
}

func TestNewTargetsOutsideRepository(t *testing.T) {
	testkit.Isolate(t)

	_, discoverError := NewTargets(context.Background(), process.System{}, t.TempDir())
	if discoverError == nil {
		t.Fatal("want an error outside a repository")
	}
}

func TestTargetsMatch(t *testing.T) {
	targets := &Targets{Root: "/repo", Folders: []string{".", "packages/api", "services/api", "packages/web"}}

	cases := map[string]string{".": ".", "packages/api": "packages/api", "packages/api/": "packages/api", "./packages/web": "packages/web", "web": "packages/web"}

	for name, want := range cases {
		folder, matchError := targets.Match(name)
		if matchError != nil || folder != want {
			t.Fatalf("%q: got %q %v", name, folder, matchError)
		}
	}

	_, ambiguousError := targets.Match("api")
	if ambiguousError == nil || !strings.Contains(ambiguousError.Error(), "ambiguous: packages/api, services/api") {
		t.Fatalf("api must be ambiguous: %v", ambiguousError)
	}

	_, unknownError := targets.Match("nope")
	if unknownError == nil || !strings.Contains(unknownError.Error(), "targets: ., packages/api") {
		t.Fatalf("an unknown target lists the targets: %v", unknownError)
	}

	_, emptyError := (&Targets{}).Match("api")
	if !errors.Is(emptyError, config.ErrNotFound) {
		t.Fatalf("no targets: %v", emptyError)
	}
}
