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

  1) patch    → 1.2.4
  2) minor    → 1.3.0
  3) major    → 2.0.0
  4) current  → 1.2.3
```

`shipo minor` on the command line wins over `level`.

## Commit and Tag

- The commit holds the version files and the lock file. Its message takes [keywords](keywords.md), but not `{hash}`.
- The tag is `v{version}`, `<folder>/v{version}` in a subfolder, or the pattern for git presets.
- `current` writes nothing and makes no commit or tag; publish runs again.

## Safety

- The tree must be clean, with no untracked files.
- The new tag must be free, locally and on the remote.
- If a later local step fails, the commit and tag are undone. An existing tag is never deleted.
