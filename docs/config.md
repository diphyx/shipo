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

- shipo uses the nearest `.shipo`, from the current folder up.
- Paths in it are relative to its folder.

!> `.shipo` is committed: never put tokens in it. Each tool keeps its own login.
