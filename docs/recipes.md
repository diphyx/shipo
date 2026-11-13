# Recipes

> `.shipo` files for common projects

## npm Package

```toml
[version]
preset = "javascript"

[build]
command = ["npm ci", "npm test", "npm run build"]

[publish]
release = true
command = "npm publish --access public"
```

## Go CLI

The tag push starts your release workflow; nothing else is needed.

```toml
[version]
preset = "go"

[build]
command = ["go vet ./...", "go test ./..."]
```

## Go with GoReleaser

shipo tags and pushes; GoReleaser builds the binaries and creates the GitHub Release, so leave `release` off. It reads `GITHUB_TOKEN` from your environment.

```toml
[version]
preset = "go"

[build]
command = ["go test ./...", "goreleaser check"]

[publish]
command = "goreleaser release --clean"
```

To run GoReleaser in CI instead, drop `[publish]` `command`: the tag push starts its workflow.

## Python Package

```toml
[version]
preset = "python"

[build]
command = "uv build"

[publish]
release = "dist/*"
command = "uv publish"
```

## Rust Crate

```toml
[version]
preset = "rust"

[build]
command = "cargo test"

[publish]
command = "cargo publish"
```

## Docker and Deploy

```toml
[version]
source  = "VERSION"
pattern = "{version}"

[publish]
workflow = "image.yml version={version}"
command  = "ssh deploy@203.0.113.10 'cd /app && export TAG={version} && docker compose pull && docker compose up -d --wait'"
```

## Monorepo

```toml
# api/.shipo: tags api/v1.2.3
[version]
preset  = "go"
pattern = "api/v{version}"
```

```toml
# web/.shipo: tags web/v1.2.3
[version]
preset = "javascript"
```

```bash
cd web && shipo patch
```

## Local Only

```toml
[publish]
push = false
```
