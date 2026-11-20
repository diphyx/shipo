package release

import (
	"context"
	"testing"

	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/tool/process"
)

func TestPushKeywords(t *testing.T) {
	repo := testkit.NewRepo(t, "[publish]\npush = [\"{remote}/{branch}\", \"origin/{branch}\"]\n", map[string]string{"package.json": PACKAGE})

	repo.Git("checkout", "--quiet", "-b", "next")
	repo.Git("push", "--quiet", "-u", "origin", "next")

	target := openTarget(t, repo, repo.Work)

	remote, branch, isPushed := target.Push()
	if !isPushed || remote != "origin" || branch != "next" {
		t.Fatalf("Push = %q %q %v", remote, branch, isPushed)
	}

	if target.Config.Publish.Push.Items[1] != "origin/next" {
		t.Fatalf("second entry = %q", target.Config.Publish.Push.Items[1])
	}
}

func TestPushKeywordErrors(t *testing.T) {
	cases := map[string]string{
		"no upstream": "{remote}/{branch}",
		"unknown":     "origin/{version}",
	}

	for name, push := range cases {
		t.Run(name, func(t *testing.T) {
			repo := testkit.NewRepo(t, "[publish]\npush = \""+push+"\"\n", map[string]string{"package.json": PACKAGE})

			_, openError := Open(context.Background(), process.System{}, repo.Work)
			if openError == nil {
				t.Fatalf("push %q: want an error", push)
			}
		})
	}

	detached := testkit.NewRepo(t, "[publish]\npush = \"origin/{branch}\"\n", map[string]string{"package.json": PACKAGE})
	detached.Git("checkout", "--quiet", "--detach")

	_, detachedError := Open(context.Background(), process.System{}, detached.Work)
	if detachedError == nil {
		t.Fatal("detached HEAD: want an error")
	}
}
