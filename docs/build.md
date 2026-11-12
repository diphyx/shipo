# Build

> Make the release files, after the tag and before anything is sent

```toml
[build]
dir         = "web"
command     = ["npm ci", "npm run build"]
environment = ["VITE_APP_VERSION={version}"]
```

## Commands

Each command runs **directly, without a shell**, the same on every OS:

| Write                              | Means                           |
| ---------------------------------- | ------------------------------- |
| `go build -ldflags "-s -w" .`      | `"…"` or `'…'` groups an argument |
| `docker build -t app:{version} .`  | [keywords](keywords.md) work    |
| `["make", "make test"]`            | several commands, in order      |
| `sh scripts/build.sh`              | for `&&`, pipes and `$VAR`      |

While a command runs, one line follows its latest output; when it ends, only the result stays:

```shipo
… build      [2/2] npm run build  › vite v5.4.0 building for production...
```

```shipo
✓ build      [1/2] npm ci
✓ build      [2/2] npm run build
```

`environment` adds only the variables you list.

## Rules

- `dir` and every program are checked before any change.
- A failing command stops the rest and undoes the bump.
- The build must not change tracked files. New files such as `dist/` are fine.
