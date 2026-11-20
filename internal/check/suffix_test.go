package check

import (
	"context"
	"testing"

	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/text/semver"
)

func TestSuffix(t *testing.T) {
	repo := testkit.NewRepo(t, "[bump]\nsuffix = \"next\"\n", map[string]string{"package.json": PACKAGE})
	repo.Git("checkout", "--quiet", "-b", "next")

	items := Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Everything())
	failure(t, items, "suffix", "origin/main would get v1.0.1-next")

	python := testkit.NewRepo(t, "[bump]\nsuffix = \"next\"\n[publish]\npush = \"{remote}/{branch}\"\n", map[string]string{"pyproject.toml": "[project]\nversion = \"1.0.0\"\n"})
	python.Git("checkout", "--quiet", "-b", "next")
	python.Git("push", "--quiet", "-u", "origin", "next")

	pythonItems := Run(context.Background(), planned(t, python, testkit.NewRunner(), semver.LEVEL_PATCH), Options{IsBump: true})
	failure(t, pythonItems, "suffix", "python versions can't end in -next")

	repo.Git("checkout", "--quiet", "main")

	mainItems := Run(context.Background(), planned(t, repo, testkit.NewRunner(), semver.LEVEL_PATCH), Options{IsBump: true})
	if _, isFound := item(mainItems, "suffix"); isFound {
		t.Fatalf("main has no suffix check, got %+v", mainItems)
	}
}
