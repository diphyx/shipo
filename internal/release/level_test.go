package release

import (
	"context"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/text/semver"
)

func TestLevels(t *testing.T) {
	repo := testkit.NewRepo(t, SUFFIX, map[string]string{"package.json": PACKAGE})
	main := openTarget(t, repo, repo.Work)

	repo.Git("checkout", "--quiet", "-b", "next")
	next := openTarget(t, repo, repo.Work)

	plain := semver.Version{Major: 1, Minor: 2}
	preview := semver.Version{Major: 1, Minor: 2, Suffix: "-next"}

	cases := []struct {
		name    string
		target  *Target
		current semver.Version
		want    string
	}{
		{"next at 1.2.0", next, plain, "patch minor major"},
		{"next at 1.2.0-next", next, preview, "final patch minor major current"},
		{"main at 1.2.0-next", main, preview, "final patch minor major"},
		{"main at 1.2.0", main, plain, "patch minor major current"},
	}

	for _, test := range cases {
		var names []string
		for _, level := range test.target.Levels(test.current) {
			names = append(names, string(level))
		}

		if strings.Join(names, " ") != test.want {
			t.Fatalf("%s: %v, want %s", test.name, names, test.want)
		}
	}

	_, finalError := next.Bump(plain, semver.LEVEL_FINAL)
	_, currentError := main.Bump(preview, semver.LEVEL_CURRENT)

	if finalError == nil || currentError == nil {
		t.Fatalf("want final of a plain version on next and current of a -next version on main to fail: %v %v", finalError, currentError)
	}

	if !next.IsStableFinal(semver.LEVEL_FINAL) || main.IsStableFinal(semver.LEVEL_FINAL) {
		t.Fatal("only final on next releases on the stable branch")
	}

	_, planError := NewRelease(context.Background(), next, semver.LEVEL_FINAL)
	if planError == nil || !strings.Contains(planError.Error(), "run shipo final") {
		t.Fatalf("final can't be planned on next: %v", planError)
	}
}
