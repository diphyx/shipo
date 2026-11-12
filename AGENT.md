# AGENT.md — shipo

shipo is a Go CLI that cuts a release: version → bump → build → publish.
[README.md](README.md) describes the behaviour, and the tests pin it down. Read both before changing a package.

## Layout

```
main.go               the binary
internal/
├── cli/              commands: flow, steps, info
├── config/           .shipo: toml.go, yaml.go, decode.go
├── release/          the shared model: target, release, keyword values
├── scope/            one package for each .shipo scope
│   ├── version/      presets, patterns, locate_*.go per format, read and write, lock files
│   ├── bump/         write, commit, tag, revert
│   ├── build/        [build]
│   └── publish/      push, release, workflow, command
├── check/            checks and release state
├── tool/             wrappers around external programs
│   ├── git/
│   ├── github/
│   └── process/      split arguments, spawn without a shell
├── text/             small pure helpers
│   ├── keyword/      {name} expansion
│   └── semver/       parse, bump, compare
├── ui/               output in color or plain, menu, confirm, terminal check
├── skill/            the embedded SKILL.md
└── testkit/          test helpers: a git repo with a bare remote, a fake runner
```

Each `scope/` package is one `.shipo` scope: `[version]`, `[bump]`, `[build]`, `[publish]`.

## Testing

- Every package has tests. They use real git through `testkit.NewRepo`, and fake `gh`, `npm` and other tools with `testkit.NewRunner`, so nothing reaches GitHub or a registry.
- `internal/cli/flow_test.go` and `steps_test.go` run whole releases end to end.
- The diphyx rules skip `_test.go` files.
- Before a change is done: `make test` and `make lint`.

## Conventions

Go code follows the [diphyx rules](https://github.com/diphyx/golangci-plugin/blob/main/AGENT.md), checked by `make lint`:

- Constants are `ALL_CAPS`. No abbreviations: `context`, `command`, `config`, not `ctx`, `cmd`, `cfg`.
- Receivers are the camelCase type name. Error variables are `{name}Error`. Booleans start with `is`, `has`, `can`.
- No `if` init statements. Blank line before `return` and after `defer`. Braces around every `switch` case.
- Exported functions have a doc comment.

## Commands

```
make init     # go mod tidy
make format   # gofmt
make test     # go test ./...
make lint     # go vet, then golangci-lint with the diphyx plugin (builds ./custom-gcl once)
make build    # ./shipo
make package  # the release archives into dist/: make package TAG=v1.2.3
make publish  # release shipo with shipo
```

## Docs

`docs/` is a [docsify](https://docsify.js.org) site, served with `npx docsify-cli serve docs`. One page per topic, listed in `docs/_sidebar.md`; `docs/AGENTS.md` is the one-page reference for AI agents, and `internal/skill/SKILL.md` is its short form, installed by `shipo skill`. Keep the three in step when behaviour changes.

Show CLI output in ` ```shipo ` blocks: `docs/index.html` colors them like the terminal.

## Release

`make publish` runs shipo on itself (`.shipo`): it bumps the git tag and pushes it. The tag push starts `.github/workflows/release.yml`:

1. `test.yml`: vet, lint and every test.
2. `.github/scripts/package.sh` builds the archives and `checksums.txt`; `gh release create` publishes them.
3. `.github/scripts/formula.sh` writes `Formula/shipo.rb` into `diphyx/homebrew-tap`, so `brew install diphyx/tap/shipo` gets it. Prereleases skip this step.

The tap step needs a `TAP_TOKEN` secret: a fine-grained token with Contents read and write on `diphyx/homebrew-tap`.
