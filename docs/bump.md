# Bump

> The next version, its commit, and its tag

```toml
[bump]
level  = "ask"
commit = "chore: release v{version}"
suffix = ["next"]
```

## Level

| `level`   | `1.2.3` →        |
| --------- | ---------------- |
| `ask`     | the menu         |
| `major`   | `2.0.0`          |
| `minor`   | `1.3.0`          |
| `patch`   | `1.2.4`          |
| `current` | `1.2.3`, again   |

```shipo
$ shipo
javascript (1.2.3)

  patch    → 1.2.4
❯ minor    → 1.3.0
  major    → 2.0.0
  current  → 1.2.3
```

Move with `↑` `↓` and press `enter`, or press `1`–`4`. Without a terminal, a number is typed instead.

`shipo minor` on the command line wins over `level`.

## Commit and Tag

- When the push branch has commits HEAD lacks, the bump merges them first, as long as they merge with no conflict; `shipo check` shows `merged before the bump`. A conflict stops the release before any change. It never rebases or force-pushes.
- If the merged commits change the version, the merge is undone and the release stops: run shipo again.

- The commit holds the version files and the lock file. Its message takes [keywords](keywords.md), but not `{hash}`.
- The tag is `v{version}`, `<folder>/v{version}` in a subfolder, or the pattern for git presets.

## Suffix

Branches that release on their own line, such as `next` beside `main`, add their name to the version:

```toml
[bump]
suffix = ["next", "release/*"]   # a branch, a glob, a list, or false

[publish]
push = "{remote}/{branch}"
```

| Branch        | `1.2.0` + `minor`   | Tag                  |
| ------------- | ------------------- | -------------------- |
| `main`        | `1.3.0`             | `v1.3.0`             |
| `next`        | `1.3.0-next`        | `v1.3.0-next`        |
| `release/2.x` | `1.3.0-release-2.x` | `v1.3.0-release-2.x` |

- The suffix goes in the version files too, so packages and images don't collide. A `/` becomes `-`.
- A listed branch bumps from its own version: `1.3.0-next` + `patch` is `1.3.1-next`.
- On another branch, a merged pre-release is finished: `1.3.0-next` + `minor` is `1.3.0`, as npm does.
- With git presets, each branch reads only its own tags: `next` the `-next` ones, `main` the ones with no suffix.
- `{previous_tag}` is the branch's own, or on a first `-next` release, the release it branched from.
- A suffix branch must push to itself; `shipo check` stops a push to another branch.
- Python and PHP take only some names: name the branch `alpha`, `beta` or `rc`.
- A detached HEAD can't say its branch: a bump stops.

## Current

`current` writes no version and makes no commit. It puts the version's tag on HEAD, and publish runs again:

| The tag is         | `current`       |
| ------------------ | --------------- |
| on HEAD            | kept            |
| missing            | created on HEAD |
| on an older commit | moved to HEAD   |

```shipo
- version    1.2.0 (current)
✓ tag        v1.2.0 moved to HEAD
✓ push       origin/main + v1.2.0
```

Use it to fix a release whose CI failed, without a new version: HEAD's new commits join this version. The push deletes the tag on the remote first, so it's a new tag event and tag workflows run again.

!> A moved tag is a changed tag: anyone who fetched the old one keeps it until `git fetch --tags --force`.

## Safety

- The tree must be clean, with no untracked files.
- A new version's tag must be free, locally and on the remote.
- If a later local step fails, the commit and tag are undone, and a moved tag goes back to its commit.
- Only `current` moves an existing tag.
