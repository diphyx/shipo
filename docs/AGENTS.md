# Agents

> One-page reference for AI agents that set up or run shipo

- **Install:** `brew install diphyx/tap/shipo`, or `go install github.com/diphyx/shipo@latest`
- **Needs:** `git`; `gh`, logged in, for `release` and `workflow`
- **Config:** `.shipo` in the folder that is released, TOML or YAML

## Model

```
checks → version → bump → build → publish
└──────────── local ────────────┘  remote
```

| Step    | Does                                       | On failure          |
| ------- | ------------------------------------------ | ------------------- |
| checks  | tools, logins, tree, remote, tag, files    | nothing changed     |
| bump    | write the version, lock file, commit, tag  | undone              |
| build   | `[build]` commands                         | bump undone         |
| publish | push → release → workflow → command        | never undone        |

## .shipo

```toml
[version]
preset  = "auto"                 # auto javascript python rust php dart java csharp go swift git custom
source  = "package.json"         # a file or a glob
pattern = "version"              # a field, a text pattern with {version}, or /regex/

[bump]
level  = "ask"                   # ask major minor patch current
commit = "chore: release v{version}"

[build]
dir         = "."
command     = ["npm ci", "npm run build"]   # no shell: no &&, use a list
environment = ["APP_VERSION={version}"]

[publish]
push     = "origin/main"         # "remote/branch", a list, or false
release  = "dist/*"              # true, a path, a list, or false
workflow = "build.yml version={tag}"
command  = "npm publish"
```

Keywords: `{version}` `{previous}` `{major}` `{minor}` `{patch}` `{tag}` `{previous_tag}` `{bump}` `{hash}` `{date}` `{branch}` `{remote}` `{target}`.

## Commands

```bash
shipo check                 # checks, release state, plan; exit 1 on a problem
shipo version               # target, preset, source, version, tag
shipo bump patch            # write, commit, tag
shipo build
shipo push
shipo release               # needs the tag on the remote
shipo workflow --no-wait
shipo command
```

With several `.shipo` in the repo, give the target: `shipo -t packages/api check`, `shipo -t api bump patch`. Without `-t`, steps use the nearest `.shipo` from the current folder.

`shipo` and `shipo <level>` are the interactive flow: they need a person.

`current` releases the same version again: no write, no commit; the tag goes on HEAD (moved from an older commit, if needed) and is pushed again, so tag workflows rerun. Moving a published tag affects anyone who fetched it: ask the user before `shipo bump current`.

## The Skill

`shipo skill <agents|claude|codex|cursor|copilot> <project|user>` installs this model as a `SKILL.md`.

## Rules

1. Read the project: stack files, `Makefile`, scripts, `.github/workflows/`.
2. Write `.shipo`, setting only what differs from the defaults.
3. Run `shipo check` until it passes.
4. Show the user `.shipo` and the plan.
5. Release with the steps. **Ask before each publish step**: it leaves the machine and isn't undone.

- Never run `shipo` or `shipo <level>`.
- Never put secrets in `.shipo`.
- After a failure, run `shipo check`, then the missing step again.
