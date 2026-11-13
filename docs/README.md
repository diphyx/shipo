# shipo

**Ship out of the box**

> Bump, tag, push, release: the tasks you repeat by hand, in one safe, easy command

![checks, version, bump and build run locally and are undone on failure; publish sends the release out and is never undone](_media/flow.svg)

```shipo
$ shipo
javascript (1.4.0)

  patch    → 1.4.1
❯ minor    → 1.5.0
  major    → 2.0.0
  current  → 1.4.0

✓ bump       minor · 1.4.0 → 1.5.0
✓ checks     tools, tree, remote, build
✓ version    package.json → 1.5.0
✓ commit     chore: release v1.5.0
✓ tag        v1.5.0
✓ build      [1/1] npm run build
✓ push       origin/main + v1.5.0

command    npm publish
Run? [y/N] y
✓ command    [1/1] npm publish
```

## What It Is

The tasks of a release, wrapped in one command, on top of the tools you already use:

- **shipo itself:** the checks, the version, the commit and the tag, and the undo when a local step fails.
- **Your tools, run or started by shipo:** the build (`npm run build`, GoReleaser, `docker`), `git push`, the GitHub Release (`gh`), your Actions workflows, and your publish (`npm publish`, a deploy).

## What It Is Not

- **Not a build tool.** It compiles, packs and uploads nothing itself. GoReleaser, Docker and your package manager keep that job; shipo runs them.
- **Not CI.** It runs on your machine, for you or an agent. What runs on GitHub stays in GitHub Actions; shipo starts it.
- **Not automatic versioning.** You pick the bump. semantic-release and release-please pick it from commit messages, in CI.
- **Not a changelog writer.** A GitHub Release gets the notes GitHub generates; a changelog file stays with your own tool.

## Why

- **Out of the box.** One binary. Presets find the version in your stack, so a few lines of config are enough, and often none.
- **One flow for every repo.** It replaces each project's own `release.sh`.
- **Safe.** Every check runs before anything changes, and a failed local step is undone.
- **Asks before it sends.** A GitHub Release, a workflow and your commands each ask first.
- **Any stack.** JavaScript, Python, Rust, PHP, Dart, Java, C#, Go, Swift, or any file by pattern.
- **For people and agents.** Interactive for you; every step also runs on its own for scripts and AI.

## Good Fit

| Yes                                                        | No                                                  |
| ---------------------------------------------------------- | --------------------------------------------------- |
| anything versioned `MAJOR.MINOR.PATCH`                     | dates or build numbers as versions                  |
| a person choosing the bump                                 | the bump chosen from commit messages                |
| monorepos where each folder releases on its own            | many packages released together in one step         |

Start with [Get Started](start.md).
