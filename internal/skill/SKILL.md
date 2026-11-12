---
name: shipo
description: Write a .shipo release config for a project and release it step by step with the shipo CLI. Use when the user wants to set up releases, bump a version, or publish a release with shipo.
---

# shipo

shipo cuts a release: checks → version → bump → build → publish. Checks change nothing; bump and build are undone if a later local step fails; publish leaves the machine and is never undone.

One `.shipo` for each folder that is released, in TOML (shown) or YAML. Every key is optional.

## .shipo

```toml
[version]
preset  = "auto"                  # auto | javascript | python | rust | php | dart | java | csharp | go | swift | git | custom
source  = "package.json"          # a file or a glob, relative to the .shipo folder
pattern = "version"               # a field path, a text pattern with {version}, or /regex/

[bump]
level  = "ask"                    # ask | major | minor | patch | current
commit = "chore: release v{version}"

[build]
dir         = "."
command     = ["npm ci", "npm run build"]   # no shell: no &&, pipes or $VAR; use a list or a script
environment = ["APP_VERSION={version}"]

[publish]
push     = "origin/main"          # "remote/branch" | a list | false
release  = "dist/*"               # true | a path | a list | false
workflow = "build.yml version={tag}"        # needs on: workflow_dispatch; only these inputs are sent
command  = ["npm publish"]                  # no shell, like build
```

Keywords: `{version}`, `{previous}`, `{major}`, `{minor}`, `{patch}`, `{tag}`, `{previous_tag}`, `{bump}`, `{hash}`, `{date}`, `{branch}`, `{remote}`, `{target}`.

## Flow

1. Read the project: stack files, `Makefile`, scripts, `.github/workflows/`.
2. Write `.shipo`, setting only what differs from the defaults.
3. Run `shipo check`, and fix until it passes.
4. Show the user the `.shipo` and the plan from `shipo check`.
5. Release with the steps. Ask the user before each publish step:

```
shipo bump patch          # write the version, commit, tag
shipo build
shipo push
shipo release             # needs the tag on the remote
shipo workflow --no-wait  # needs the tag on the remote
shipo command
```

## Rules

- Never run `shipo` or `shipo <level>`; they are interactive and need a person.
- Never put secrets in `.shipo`; it is committed. Each tool keeps its own login.
- `release` and `workflow` need `push`; with `push = false` they are skipped.
- `shipo bump current` re-releases the same version: it moves the tag to HEAD and re-pushes it. Ask the user before running it.
- After a failure, run `shipo check` to see where the release stands, then run the missing step again.
- If unsure, ask the user.
