# Version

> Where the version lives

```toml
[version]
preset  = "javascript"
source  = "package.json"
pattern = "version"
```

A preset is a ready `source` and `pattern`; set either one to change it.

## Presets

| `preset`     | Reads                                          |
| ------------ | ---------------------------------------------- |
| `javascript` | `package.json` → `version`                     |
| `python`     | `pyproject.toml` → `project.version` or `tool.poetry.version` |
| `rust`       | `Cargo.toml` → `package.version` or `workspace.package.version` |
| `php`        | `composer.json` → `version`                    |
| `dart`       | `pubspec.yaml` → `version`                     |
| `java`       | `pom.xml` → `project.version`                  |
| `csharp`     | `*.csproj` → `PropertyGroup.Version`           |
| `go`         | git tags `v{version}`                          |
| `swift`      | git tags `{version}`                           |
| `git`        | git tags `v{version}`                          |
| `custom`     | your `source` and `pattern`                    |

- `auto`, the default, detects the one stack in the folder. With only `source`, it picks the preset from the file name.
- Only the version text is rewritten: formatting and comments stay.
- With git tags, the version is the highest matching tag, or `0.0.0`. A bump adds a tag and no commit.

## Pattern

| Source                          | `pattern` is   | Example               |
| ------------------------------- | -------------- | --------------------- |
| `.json` `.toml` `.yaml` `.xml`  | a field path   | `tool.poetry.version` |
| any other file                  | a text pattern | `VERSION="{version}"` |
| git tags                        | a tag pattern  | `api/v{version}`      |

A text pattern is literal, except:

| Part        | Matches              | On write        |
| ----------- | -------------------- | --------------- |
| `{version}` | the version; required | the new version |
| `{hash}`    | a commit hash        | HEAD's short hash |
| `{counter}` | a number             | plus one        |
| `{date}`    | `YYYY-MM-DD`         | today           |
| `{*}`       | any text             | kept            |
| `{_}`       | spaces, or none      | kept            |
| `^` … `$`   | a whole line         | —               |

Wrap it in `/…/` for a full regex; the keywords still work. Every match is rewritten and must hold the same version.

## Lock Files

After the write, the stack's own tool refreshes its lock file:

| Lock file                                         | Tool                    |
| ------------------------------------------------- | ----------------------- |
| `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock` | `npm`, `yarn`, `pnpm`, `bun` |
| `Cargo.lock`                                      | `cargo`                 |
| `uv.lock`, `poetry.lock`, `pdm.lock`              | `uv`, `poetry`, `pdm`   |

Two lock files of one stack in a folder stop shipo before any change.

## Examples

```toml
source = "packages/*/package.json"         # one version in many packages
```

```toml
preset  = "go"
pattern = "api/v{version}"                 # a Go module in a subfolder
```

```toml
source  = "VERSION"
pattern = "{version}"                      # a plain VERSION file
```

```toml
source  = "internal/version.go"
pattern = '/const Version = "{version}"/'  # a constant in code
```
