# Get Started

> Install, create `.shipo`, check, release

## 1. Install

| With     | Command                                     |
| -------- | ------------------------------------------- |
| Homebrew | `brew install diphyx/tap/shipo`             |
| Go       | `go install github.com/diphyx/shipo@latest` |
| binary   | a [release](https://github.com/diphyx/shipo/releases) archive, on your `PATH` |

shipo needs `git`. `gh`, logged in, is needed only for `release` and `workflow`.

## 2. Create `.shipo`

In the folder you release:

```shipo
$ shipo init
✓ init       .shipo (javascript)
```

```toml
[version]
preset = "javascript"

[bump]
level = "ask"

[publish]
push = "origin/main"
```

## 3. Check

Shows what a release would do, and changes nothing:

```shipo
$ shipo check
checks
  ✓ tools     git
  ✓ tree      clean
  ✓ remote    origin/main, fast-forward ok

plan
  push      origin/main + v1.4.1
```

## 4. Release

```bash
shipo           # choose the bump in the menu
shipo minor     # or give it
```

shipo writes the version, commits `chore: release v1.5.0`, tags `v1.5.0`, and pushes both.

## 5. Add More

```toml
[build]
command = ["npm ci", "npm run build"]

[publish]
release = "dist/*.tgz"      # a GitHub Release with the files
command = "npm publish"
```

Each publish task asks first. A failed build undoes the bump; after the push, nothing is undone.

Next: [.shipo](config.md) · [Commands](commands.md) · [Recipes](recipes.md)
