# Bump

> The next version, its commit, and its tag

```toml
[bump]
level  = "ask"
commit = "chore: release v{version}"
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

- The commit holds the version files and the lock file. Its message takes [keywords](keywords.md), but not `{hash}`.
- The tag is `v{version}`, `<folder>/v{version}` in a subfolder, or the pattern for git presets.

## Current

`current` writes no version and makes no commit. It puts the version's tag on HEAD, and publish runs again:

| The tag is            | `current`                                         |
| --------------------- | ------------------------------------------------- |
| on HEAD               | kept                                              |
| missing               | created on HEAD                                   |
| on an older commit    | moved to HEAD, after a confirm                    |

```shipo
Move tag v1.2.0 from 6fe69cc to HEAD (a58a190)? [y/N] y
- version    1.2.0 (current)
✓ tag        v1.2.0 moved to HEAD
✓ push       origin/main + v1.2.0
```

Use it to fix a release whose CI failed, without a new version: HEAD's new commits join this version. The push deletes the tag on the remote first, so it's a new tag event and tag workflows run again.

!> A moved tag is a changed tag: anyone who fetched the old one keeps it until `git fetch --tags --force`. `shipo bump current` moves it without asking.

## Safety

- The tree must be clean, with no untracked files.
- A new version's tag must be free, locally and on the remote.
- If a later local step fails, the commit and tag are undone, and a moved tag goes back to its commit.
- Only `current` moves an existing tag, and the flow asks first.
