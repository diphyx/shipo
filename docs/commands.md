# Commands

```bash
shipo                           # the whole flow, with the bump menu
shipo major|minor|patch|current # the whole flow, bump given

shipo bump <level>              # steps: one task each, no prompts
shipo build
shipo push
shipo release
shipo workflow [--no-wait]
shipo command

shipo version                   # info
shipo check
shipo init
shipo skill [tool] [project|user]
```

Exit code `0` on success, `1` on an error, printed as `error: …`.

## Flow

`shipo` runs checks → version → bump → build → publish, and asks before each publish task. It needs a terminal.

If HEAD is a release that never reached the remote, it offers to publish that one instead of a new bump.

## Steps

No prompts and no terminal needed, for scripts and agents:

```bash
shipo bump patch       # write, commit, tag
shipo build
shipo push
shipo release          # needs the tag on the remote
shipo workflow         # needs the tag on the remote
shipo command
```

Every step but `bump` works on the release HEAD already is, so running one again is how a failed publish is retried.

## Info

| Command        | Shows                                                         |
| -------------- | ------------------------------------------------------------- |
| `shipo version` | target, preset, source, version, tag                         |
| `shipo check`  | the checks, where the release stands, and the plan; changes nothing |
| `shipo init`   | writes a `.shipo` for the detected stack                      |
| `shipo skill`  | installs the [AI skill](AGENTS.md#the-skill)                  |

```shipo
$ shipo check
checks
  ✓ tools     gh, git, npm
  ✓ remote    origin/main, fast-forward ok

release v1.5.0
  ✓ tag       local, remote
  ✗ release   not created

plan
  push      origin/main + v1.5.0
  release   v1.5.0 + dist/*
  command   npm publish
```

```shipo
$ shipo version
target   web/
preset   javascript
source   package.json
version  1.4.0
tag      web/v1.4.0
```

`skill` takes a tool, `agents` `claude` `codex` `cursor` `copilot`, and `project` or `user`. Without them, it asks.

## Output

| Mark | Means   |
| ---- | ------- |
| `✓`  | done    |
| `…`  | running |
| `-`  | skipped |
| `✗`  | failed  |

While a program runs, its latest line shows on the step's line, and only the result stays. If it fails, its last lines are shown under the `✗`:

```shipo
✗ command    [1/1] npm publish
  │ npm notice Publishing to https://registry.npmjs.org/
  │ npm error 403 Forbidden - you must be logged in
```

On a terminal the output is in color. Piped, in a file, or with `NO_COLOR` set, it is plain text, and programs' output streams in full; `FORCE_COLOR` turns color on anywhere.
