# Keywords

> `{name}` values in commit messages and commands

| Keyword          | Value                        | Example      |
| ---------------- | ---------------------------- | ------------ |
| `{version}`      | the new version              | `1.5.0`      |
| `{previous}`     | the one before               | `1.4.2`      |
| `{major}` `{minor}` `{patch}` | its parts       | `1` `5` `0`  |
| `{tag}`          | the new tag                  | `v1.5.0`     |
| `{previous_tag}` | the tag before               | `v1.4.2`     |
| `{bump}`         | the level                    | `minor`      |
| `{hash}`         | the release commit, short    | `a1b2c3d`    |
| `{date}`         | today                        | `2026-11-12` |
| `{branch}`       | the push branch              | `main`       |
| `{remote}`       | the push remote              | `origin`     |
| `{target}`       | the `.shipo` folder          | `web`        |

- They work in `bump.commit`, `build` and `publish`. `{hash}` doesn't work in `bump.commit`.
- An unknown `{name}` stays as written; `{{` is a literal `{`.
- `version.pattern` has [its own](version.md#pattern).
