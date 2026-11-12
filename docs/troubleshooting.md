# Troubleshooting

> Run `shipo check` first: it lists every problem, and changes nothing

| Message                                         | Fix                                                |
| ----------------------------------------------- | -------------------------------------------------- |
| `tree: has changes; commit or stash them`       | commit, stash, or remove them, untracked files too |
| `tools: not installed: gh`                      | install it, or drop the key that needs it          |
| `tools: gh is not logged in`                    | `gh auth login`                                    |
| `remote: … has commits you don't have`          | `git pull`                                         |
| `tag: v1.5.0 already exists`                    | that version is out; bump further                  |
| `lock: … has several lock files`                | delete the one you don't use                       |
| `workflow: build.yml has no on: workflow_dispatch` | add `workflow_dispatch:` under `on:`            |
| `no .shipo found in this folder or above`       | `shipo init`, or `cd` into the target              |
| `unknown key bump.levl`                         | fix the spelling; see [keys](config.md#keys)       |
| `several stacks detected (…)`                   | set `preset`                                       |
| `pyproject.toml: the version is dynamic`        | point `source` at the file that holds it           |
| `versions differ: …`                            | make every file hold the same version              |
| `build changed tracked files: …`                | write output to `dist/`, not to tracked files      |
| `needs a terminal`                              | run `shipo` yourself, or use the steps             |
| `HEAD is not a release`                         | `shipo bump <level>` first                         |
| `tag v1.5.0 is not on origin; run shipo push first` | `shipo push`                                   |
| `✗ … failed; nothing was undone`                | fix it, then run that step again                   |
| `tag v1.2.0 not moved; nothing changed`         | you said no to moving the tag; bump to a new version instead |

## Common Mistakes

- **`&&` in a command:** there's no shell. Use a list, or a script.
- **`push = true`:** name the branch, `"origin/main"`.
- **A token in `.shipo`:** it's committed. Use each tool's login.
