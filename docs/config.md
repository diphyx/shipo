# .shipo

> One file for each folder you release, in TOML or YAML

## Keys

Every scope and key is optional.

| Key                 | Takes                                     | Default                     |
| ------------------- | ----------------------------------------- | --------------------------- |
| [`version`](version.md) `preset`  | `auto`, a stack, `git`, `custom` | `auto`                    |
| `version` `source`  | a file or a glob                          | from the preset             |
| `version` `pattern` | a field, a text pattern, or `/regex/`     | from the preset             |
| [`bump`](bump.md) `level` | `ask` `major` `minor` `patch` `current` | `ask`                     |
| `bump` `commit`     | a message                                 | `chore: release v{version}` |
| [`build`](build.md) `dir` | a folder                            | the `.shipo` folder         |
| `build` `command`   | a command, or a list                      | none                        |
| `build` `environment` | `NAME=value`, or a list                 | none                        |
| [`publish`](publish.md) `push` | `remote/branch`, a list, or `false` | `origin/main`          |
| `publish` `release` | `true`, a path, a list, or `false`        | `false`                     |
| `publish` `workflow` | `file name=value …`, a list, or `false`  | `false`                     |
| `publish` `command` | a command, or a list                      | none                        |

Messages and commands take [keywords](keywords.md) such as `{version}`.

## TOML or YAML

The file is always `.shipo`; shipo reads TOML first, then YAML.

```toml
[version]
preset = "javascript"

[publish]
command = ["npm publish"]
```

```yaml
version:
  preset: javascript

publish:
  command:
    - npm publish
```

Values are strings, booleans and lists of strings. Anything else is an error with its line number.

## Targets

Each folder with a `.shipo` releases on its own:

```
repo/
├── .shipo          ← tags v1.2.3
├── api/.shipo      ← tags api/v1.2.3
└── web/.shipo      ← tags web/v1.2.3
```

- shipo finds every `.shipo` in the repo, skipping what git ignores (`node_modules`, `dist`, …).
- One `.shipo` is used directly. With several, `shipo`, `shipo check` and `shipo version` show a menu, from any folder: the root first, only if it has a `.shipo`, then each folder by path. The nearest `.shipo` is preselected.
- A folder deeper than three levels shows as `first/../last` in the menu.
- `-t <target>` picks one without the menu: its folder from the root, or its last folder name when only one target has it.
- Steps never show the menu: without `-t` they use the nearest `.shipo`, from the current folder up.
- Paths in a `.shipo` are relative to its folder.

```shipo
$ shipo
target

❯ packages/api   → 1.4.0
  packages/web   → 0.9.2
  tools/../cli   → 0.2.0
```

```bash
shipo -t packages/api patch     # the flow, for one target
shipo -t web bump minor         # a step, by its last folder name
```

!> `.shipo` is committed: never put tokens in it. Each tool keeps its own login.
