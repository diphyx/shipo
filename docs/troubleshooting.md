# Troubleshooting

> Run `shipo check` first: it lists every problem, and changes nothing

| Message                                         | Fix                                                |
| ----------------------------------------------- | -------------------------------------------------- |
| `tree: has changes; commit or stash them`       | commit, stash, or remove them, untracked files too |
| `tools: not installed: gh`                      | install it, or drop the key that needs it          |
| `tools: gh is not logged in`                    | `gh auth login`                                    |
| `remote: … has commits you don't have`          | they conflict with yours, or the bump is `current`: `git pull` and resolve |
| `the merged commits change the version`         | the push branch has a newer release: run shipo again |
| `tag: v1.5.0 already exists`                    | that version is out; bump further                  |
| `lock: … has several lock files`                | delete the one you don't use                       |
| `workflow: build.yml has no on: workflow_dispatch` | add `workflow_dispatch:` under `on:`            |
| `no .shipo found in this folder or above`       | `shipo init`, or `cd` into the target              |
| `no .shipo in this folder or above; pick a target with -t` | add `-t <target>`, or `cd` into the target |
| `unknown key bump.levl`                         | fix the spelling; see [keys](config.md#keys)       |
| `several stacks detected (…)`                   | set `preset`                                       |
| `pyproject.toml: the version is dynamic`        | point `source` at the file that holds it           |
| `versions differ: …`                            | make every file hold the same version              |
| `build changed tracked files: …`                | write output to `dist/`, not to tracked files      |
| `needs a terminal`                              | run `shipo` yourself, or use the steps             |
| `HEAD is not a release`                         | `shipo bump <level>` first                         |
| `tag v1.5.0 is not on origin; run shipo push first` | `shipo push`                                   |
| `✗ … failed; nothing was undone`                | fix it, then run that step again                   |
| `{remote} needs an upstream`                    | `git push -u origin <branch>` once                 |
| `{branch} and {remote} need a checked-out branch` | `git switch <branch>`; in CI, check out the branch, not the commit |
| `bump.suffix needs a checked-out branch`        | the same: a detached HEAD can't say its branch     |
| `suffix: origin/main would get v1.3.0-next`     | `push = "{remote}/{branch}"`                       |
| `suffix: python versions can't end in -next`    | name the branch `alpha`, `beta` or `rc`            |
| `current: 1.3.0-next is a pre-release from another branch` | `final` releases it as `1.3.0`          |
| `final: 1.2.0 is below 1.3.0, the last release on main` | `main` is ahead: bump `next`, then `shipo final` |
| `final releases on main: run shipo final`       | `shipo final` in a terminal; the steps never switch branches |
| `next and main conflict`                        | merge `main` into `next`, resolve, then `shipo final` |

## Common Mistakes

- **`&&` in a command:** there's no shell. Use a list, or a script.
- **`push = true`:** name the branch, `"origin/main"`.
- **A token in `.shipo`:** it's committed. Use each tool's login.
