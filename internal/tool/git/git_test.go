package git

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/diphyx/shipo/internal/testkit"
	"github.com/diphyx/shipo/internal/tool/process"
)

func open(t *testing.T) (*testkit.Repo, *Git, context.Context) {
	t.Helper()

	repo := testkit.NewRepo(t, "", map[string]string{"a.txt": "a\n"})

	return repo, NewGit(process.System{}, repo.Work), context.Background()
}

func TestRepositoryInfo(t *testing.T) {
	repo, git, background := open(t)

	root, rootError := git.Root(background)
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	resolvedWork, _ := filepath.EvalSymlinks(repo.Work)

	if rootError != nil || resolvedRoot != resolvedWork {
		t.Fatalf("Root = %q, %v", root, rootError)
	}

	head, _ := git.Head(background)
	short, _ := git.ShortHash(background, "HEAD")

	if len(head) != 40 || !strings.HasPrefix(head, short) {
		t.Fatalf("Head = %q, ShortHash = %q", head, short)
	}

	branch, _ := git.Branch(background)
	subject, _ := git.Subject(background, "HEAD")

	if branch != "main" || subject != "init" {
		t.Fatalf("Branch = %q, Subject = %q", branch, subject)
	}

	if git.Resolve(background, "main") != head || git.Resolve(background, "nope") != "" {
		t.Fatal("Resolve")
	}

	url, urlError := git.RemoteURL(background, "origin")
	if urlError != nil || url != repo.Remote {
		t.Fatalf("RemoteURL = %q, %v", url, urlError)
	}

	if !git.HasRemote(background, "origin") || git.HasRemote(background, "nope") {
		t.Fatal("HasRemote")
	}
}

func TestNotARepository(t *testing.T) {
	testkit.Isolate(t)

	_, rootError := NewGit(process.System{}, t.TempDir()).Root(context.Background())
	if rootError == nil || rootError.Error() != "not a git repository" {
		t.Fatalf("want not a git repository, got %v", rootError)
	}
}

func TestCleanAndChanged(t *testing.T) {
	repo, git, background := open(t)

	isClean, _ := git.IsClean(background)
	changed, _ := git.ChangedTracked(background)

	if !isClean || len(changed) != 0 {
		t.Fatalf("clean = %v, changed = %v", isClean, changed)
	}

	repo.Write(map[string]string{"untracked.txt": "x"})

	isClean, _ = git.IsClean(background)
	changed, _ = git.ChangedTracked(background)

	if isClean || len(changed) != 0 {
		t.Fatal("an untracked file makes the tree dirty, but isn't a tracked change")
	}

	repo.Write(map[string]string{"a.txt": "changed\n"})

	changed, _ = git.ChangedTracked(background)
	if !reflect.DeepEqual(changed, []string{"a.txt"}) {
		t.Fatalf("changed = %v", changed)
	}
}

func TestCommitTagAndReset(t *testing.T) {
	repo, git, background := open(t)
	base, _ := git.Head(background)

	repo.Write(map[string]string{"a.txt": "b\n"})

	addError := git.Add(background, "a.txt")
	commitError := git.Commit(background, "chore: release v1.0.0", false)

	if addError != nil || commitError != nil {
		t.Fatalf("add %v, commit %v", addError, commitError)
	}

	emptyError := git.Commit(background, "empty", false)
	if emptyError == nil {
		t.Fatal("an empty commit must fail unless allowed")
	}

	allowedError := git.Commit(background, "empty", true)
	if allowedError != nil {
		t.Fatal(allowedError)
	}

	tagError := git.Tag(background, "v1.0.0", "Release v1.0.0")
	if tagError != nil || !git.HasTag(background, "v1.0.0") || git.HasTag(background, "v9.9.9") {
		t.Fatalf("Tag: %v", tagError)
	}

	duplicateError := git.Tag(background, "v1.0.0", "again")
	if duplicateError == nil {
		t.Fatal("a duplicate tag must fail")
	}

	tags, _ := git.Tags(background)
	at, _ := git.TagsAt(background, "HEAD")

	if !reflect.DeepEqual(tags, []string{"v1.0.0"}) || !reflect.DeepEqual(at, []string{"v1.0.0"}) {
		t.Fatalf("Tags = %v, TagsAt = %v", tags, at)
	}

	if !git.IsAncestor(background, base, "HEAD") || git.IsAncestor(background, "HEAD", base) {
		t.Fatal("IsAncestor")
	}

	deleteError := git.DeleteTag(background, "v1.0.0")
	resetError := git.ResetHard(background, base)

	if deleteError != nil || resetError != nil {
		t.Fatalf("delete %v, reset %v", deleteError, resetError)
	}

	head, _ := git.Head(background)
	tags, _ = git.Tags(background)

	if head != base || len(tags) != 0 || repo.Read("a.txt") != "a\n" {
		t.Fatal("reset did not restore the base commit")
	}
}

func TestPushFetchAndRemoteTags(t *testing.T) {
	repo, git, background := open(t)

	repo.Write(map[string]string{"a.txt": "b\n"})
	repo.Commit("release")
	_ = git.Tag(background, "v1.0.0", "Release v1.0.0")

	isRemote, _ := git.HasRemoteTag(background, "origin", "v1.0.0")
	if isRemote {
		t.Fatal("tag must not be on the remote yet")
	}

	pushError := git.PushAtomic(background, "origin", "HEAD:refs/heads/main", "refs/tags/v1.0.0")
	if pushError != nil {
		t.Fatal(pushError)
	}

	isRemote, _ = git.HasRemoteTag(background, "origin", "v1.0.0")
	if !isRemote {
		t.Fatal("tag must be on the remote")
	}

	fetchError := git.Fetch(background, "origin")
	if fetchError != nil {
		t.Fatal(fetchError)
	}

	if git.Resolve(background, "origin/main") != git.Resolve(background, "HEAD") {
		t.Fatal("origin/main must be the pushed commit")
	}

	rejectedError := git.PushAtomic(background, "origin", "HEAD~1:refs/heads/main")
	if rejectedError == nil {
		t.Fatal("a non-fast-forward push must fail")
	}
}

func TestMoveBranch(t *testing.T) {
	repo, git, background := open(t)
	base, _ := git.Head(background)

	repo.Git("checkout", "--quiet", "-b", "feature")
	repo.Commit("work")

	moveError := git.MoveBranch(background, "main", "HEAD")
	if moveError != nil {
		t.Fatal(moveError)
	}

	if git.Resolve(background, "main") == base {
		t.Fatal("main did not move")
	}
}

func TestEmptyRepository(t *testing.T) {
	testkit.Isolate(t)

	dir := t.TempDir()
	testkit.Run(t, dir, "git", "init", "--quiet")

	git := NewGit(process.System{}, dir)
	background := context.Background()

	tags, tagsError := git.Tags(background)
	if tagsError != nil || tags != nil {
		t.Fatalf("Tags = %v, %v", tags, tagsError)
	}

	_, headError := git.Head(background)
	if headError == nil {
		t.Fatal("Head of an empty repository must fail")
	}

	_ = os.WriteFile(filepath.Join(dir, "x"), []byte("x"), 0o644)

	isClean, _ := git.IsClean(background)
	if isClean {
		t.Fatal("an untracked file in an empty repository is a change")
	}
}
