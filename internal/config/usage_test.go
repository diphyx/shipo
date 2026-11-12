package config

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// defaults is the Config an empty .shipo gives.
func defaults() Config {
	return Config{
		Version: Version{Preset: DEFAULT_PRESET},
		Bump:    Bump{Level: DEFAULT_LEVEL, Commit: DEFAULT_COMMIT},
		Publish: Publish{Push: Switch{IsSet: true, Items: []string{DEFAULT_PUSH}}},
	}
}

func items(values ...string) Switch {
	return Switch{IsSet: true, Items: values}
}

// usage lists real .shipo files, from the design and the README, each written
// in TOML and in YAML, with the Config both must give.
var usage = []struct {
	name string
	toml string
	yaml string
	want func(*Config)
}{
	{
		name: "empty",
		toml: "",
		yaml: "",
		want: func(*Config) {},
	},
	{
		name: "preset",
		toml: "[version]\npreset = \"javascript\"\n",
		yaml: "version:\n  preset: javascript\n",
		want: func(config *Config) { config.Version.Preset = "javascript" },
	},
	{
		name: "monorepo glob",
		toml: "[version]\npreset = \"javascript\"\nsource = \"packages/*/package.json\"\n",
		yaml: "version:\n  preset: javascript\n  source: packages/*/package.json\n",
		want: func(config *Config) {
			config.Version.Preset = "javascript"
			config.Version.Source = "packages/*/package.json"
		},
	},
	{
		name: "go submodule tags, fixed bump",
		toml: "[version]\npreset  = \"go\"\npattern = \"api/v{version}\"\n\n[bump]\nlevel = \"patch\"\n",
		yaml: "version:\n  preset: go\n  pattern: api/v{version}\n\nbump:\n  level: patch\n",
		want: func(config *Config) {
			config.Version = Version{Preset: "go", Pattern: "api/v{version}"}
			config.Bump.Level = "patch"
		},
	},
	{
		name: "git tags",
		toml: "[version]\npreset  = \"git\"\npattern = \"release-{version}\"\n",
		yaml: "version:\n  preset: git\n  pattern: release-{version}\n",
		want: func(config *Config) { config.Version = Version{Preset: "git", Pattern: "release-{version}"} },
	},
	{
		name: "custom file, quotes in the pattern",
		toml: "[version]\npreset  = \"custom\"\nsource  = \"src/version.sh\"\npattern = 'VERSION=\"{version}\"'\n",
		yaml: "version:\n  preset: custom\n  source: src/version.sh\n  pattern: 'VERSION=\"{version}\"'\n",
		want: func(config *Config) {
			config.Version = Version{Preset: "custom", Source: "src/version.sh", Pattern: `VERSION="{version}"`}
		},
	},
	{
		name: "regex pattern",
		toml: "[version]\nsource  = \"main.go\"\npattern = '/^const Version = \"{version}\"$/'\n",
		yaml: "version:\n  source: main.go\n  pattern: '/^const Version = \"{version}\"$/'\n",
		want: func(config *Config) {
			config.Version.Source = "main.go"
			config.Version.Pattern = `/^const Version = "{version}"$/`
		},
	},
	{
		name: "text pattern with keywords",
		toml: "[version]\nsource  = \"README.md\"\npattern = \"version-{version}-blue\"\n",
		yaml: "version:\n  source: README.md\n  pattern: version-{version}-blue\n",
		want: func(config *Config) {
			config.Version.Source = "README.md"
			config.Version.Pattern = "version-{version}-blue"
		},
	},
	{
		name: "custom commit",
		toml: "[bump]\nlevel  = \"ask\"\ncommit = \"release: {tag} ({bump})\"\n",
		yaml: "bump:\n  level: ask\n  commit: \"release: {tag} ({bump})\"\n",
		want: func(config *Config) { config.Bump.Commit = "release: {tag} ({bump})" },
	},
	{
		name: "build, one command",
		toml: "[build]\ncommand = \"make dist\"\n",
		yaml: "build:\n  command: make dist\n",
		want: func(config *Config) { config.Build.Command = List{"make dist"} },
	},
	{
		name: "build, keyword in an argument",
		toml: "[build]\ncommand = \"docker build -t app:{version} .\"\n",
		yaml: "build:\n  command: docker build -t app:{version} .\n",
		want: func(config *Config) { config.Build.Command = List{"docker build -t app:{version} ."} },
	},
	{
		name: "build, quoted argument",
		toml: "[build]\ncommand = \"go build -ldflags '-X main.version={version}' ./cmd/app\"\n",
		yaml: "build:\n  command: go build -ldflags '-X main.version={version}' ./cmd/app\n",
		want: func(config *Config) {
			config.Build.Command = List{"go build -ldflags '-X main.version={version}' ./cmd/app"}
		},
	},
	{
		name: "build, folder, list and variables",
		toml: "[build]\ndir         = \"web\"\ncommand     = [\"npm ci\", \"npm run build\"]\nenvironment = [\"APP_VERSION={version}\", \"APP_COMMIT={hash}\"]\n",
		yaml: "build:\n  dir: web\n  command:\n    - npm ci\n    - npm run build\n  environment:\n    - APP_VERSION={version}\n    - APP_COMMIT={hash}\n",
		want: func(config *Config) {
			config.Build = Build{
				Dir:         "web",
				Command:     List{"npm ci", "npm run build"},
				Environment: List{"APP_VERSION={version}", "APP_COMMIT={hash}"},
			}
		},
	},
	{
		name: "publish, other remote",
		toml: "[publish]\npush = \"upstream/release\"\n",
		yaml: "publish:\n  push: upstream/release\n",
		want: func(config *Config) { config.Publish.Push = items("upstream/release") },
	},
	{
		name: "publish, branch with a slash",
		toml: "[publish]\npush = \"origin/release/v2\"\n",
		yaml: "publish:\n  push: origin/release/v2\n",
		want: func(config *Config) { config.Publish.Push = items("origin/release/v2") },
	},
	{
		name: "publish, two remotes",
		toml: "[publish]\npush = [\"origin/main\", \"mirror/main\"]\n",
		yaml: "publish:\n  push: [origin/main, mirror/main]\n",
		want: func(config *Config) { config.Publish.Push = items("origin/main", "mirror/main") },
	},
	{
		name: "publish, local only",
		toml: "[publish]\npush = false\n",
		yaml: "publish:\n  push: false\n",
		want: func(config *Config) { config.Publish.Push = Switch{IsSet: true, IsOff: true} },
	},
	{
		name: "publish, release without files",
		toml: "[publish]\nrelease = true\n",
		yaml: "publish:\n  release: true\n",
		want: func(config *Config) { config.Publish.Release = Switch{IsSet: true, IsOn: true} },
	},
	{
		name: "publish, release with files",
		toml: "[publish]\nrelease = [\"dist/*.tar.gz\", \"checksums.txt\"]\n",
		yaml: "publish:\n  release:\n    - dist/*.tar.gz\n    - checksums.txt\n",
		want: func(config *Config) { config.Publish.Release = items("dist/*.tar.gz", "checksums.txt") },
	},
	{
		name: "publish, workflow with inputs",
		toml: "[publish]\nworkflow = [\"build.yml version={tag} title=\\\"a b\\\"\", \"docs.yml\"]\n",
		yaml: "publish:\n  workflow:\n    - build.yml version={tag} title=\"a b\"\n    - docs.yml\n",
		want: func(config *Config) {
			config.Publish.Workflow = items(`build.yml version={tag} title="a b"`, "docs.yml")
		},
	},
	{
		name: "publish, package upload",
		toml: "[publish]\ncommand = \"npm publish --access public\"\n",
		yaml: "publish:\n  command: npm publish --access public\n",
		want: func(config *Config) { config.Publish.Command = List{"npm publish --access public"} },
	},
	{
		name: "publish, deploy over ssh",
		toml: "[publish]\nworkflow = \"build.yml version={version}\"\ncommand  = \"ssh deploy@203.0.113.10 'cd /app && export TAG={version} && docker compose pull && docker compose up -d --wait'\"\n",
		yaml: "publish:\n  workflow: build.yml version={version}\n  command: ssh deploy@203.0.113.10 'cd /app && export TAG={version} && docker compose pull && docker compose up -d --wait'\n",
		want: func(config *Config) {
			config.Publish.Workflow = items("build.yml version={version}")
			config.Publish.Command = List{"ssh deploy@203.0.113.10 'cd /app && export TAG={version} && docker compose pull && docker compose up -d --wait'"}
		},
	},
	{
		name: "everything, with comments",
		toml: `# release config
[version]
preset = "javascript"   # the stack

[bump]
level  = "ask"
commit = "chore: release v{version}"

[build]
command = [
    "npm ci",          # install
    "npm run build",
]

[publish]
push     = "origin/main"
release  = "dist/*"
workflow = "build.yml version={tag}"
command  = ["npm publish"]
`,
		yaml: `# release config
version:
  preset: javascript   # the stack

bump:
  level: ask
  commit: "chore: release v{version}"

build:
  command:
    - npm ci           # install
    - npm run build

publish:
  push: origin/main
  release: dist/*
  workflow: build.yml version={tag}
  command: [npm publish]
`,
		want: func(config *Config) {
			config.Version.Preset = "javascript"
			config.Build.Command = List{"npm ci", "npm run build"}
			config.Publish.Release = items("dist/*")
			config.Publish.Workflow = items("build.yml version={tag}")
			config.Publish.Command = List{"npm publish"}
		},
	},
}

func TestUsage(t *testing.T) {
	for _, example := range usage {
		want := defaults()
		example.want(&want)

		for format, content := range map[string]string{"toml": example.toml, "yaml": example.yaml} {
			t.Run(example.name+"/"+format, func(t *testing.T) {
				got, parseError := Parse(content)
				if parseError != nil {
					t.Fatalf("Parse: %v\n%s", parseError, content)
				}

				if !reflect.DeepEqual(*got, want) {
					t.Fatalf("got  %+v\nwant %+v", *got, want)
				}
			})
		}
	}
}

// TestUsageMistakes checks the mistakes a person is likely to make get a clear
// error, in both formats.
func TestUsageMistakes(t *testing.T) {
	mistakes := []struct {
		name string
		toml string
		yaml string
	}{
		{"typo in a key", "[bump]\nlevl = \"patch\"\n", "bump:\n  levl: patch\n"},
		{"typo in a section", "[publsh]\npush = \"origin/main\"\n", "publsh:\n  push: origin/main\n"},
		{"push without a branch", "[publish]\npush = \"origin\"\n", "publish:\n  push: origin\n"},
		{"push = true", "[publish]\npush = true\n", "publish:\n  push: true\n"},
		{"workflow = true", "[publish]\nworkflow = true\n", "publish:\n  workflow: true\n"},
		{"level as a boolean", "[bump]\nlevel = true\n", "bump:\n  level: true\n"},
		{"empty command list", "[build]\ncommand = []\n", "build:\n  command: []\n"},
		{"environment without =", "[build]\ncommand = \"make\"\nenvironment = \"APP_VERSION\"\n", "build:\n  command: make\n  environment: APP_VERSION\n"},
		{"dir without command", "[build]\ndir = \"web\"\n", "build:\n  dir: web\n"},
		{"key outside a section", "level = \"patch\"\n", "level: patch\n"},
		{"key set twice", "[bump]\nlevel = \"patch\"\nlevel = \"minor\"\n", "bump:\n  level: patch\n  level: minor\n"},
	}

	for _, mistake := range mistakes {
		for format, content := range map[string]string{"toml": mistake.toml, "yaml": mistake.yaml} {
			t.Run(mistake.name+"/"+format, func(t *testing.T) {
				_, parseError := Parse(content)
				if parseError == nil {
					t.Fatalf("want an error for:\n%s", content)
				}
			})
		}
	}
}

// TestOwnShipo loads the .shipo that shipo releases itself with.
func TestOwnShipo(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")

	config, loadError := Load(filepath.Join(root, FILE_NAME))
	if loadError != nil {
		t.Fatal(loadError)
	}

	if config.Version.Preset != "go" || config.Publish.Push.Items[0] != "origin/main" || len(config.Build.Command) != 2 {
		t.Fatalf("unexpected config: %+v", config)
	}
}

const TOML_FULL = `
# release config
[version]
preset  = "custom"
source  = "src/version.sh"
pattern = 'VERSION="{version}"'   # literal string

[bump]
level  = "patch"
commit = "release: {tag} # not a comment"

[build]
dir     = "web"
command = [
    "npm ci",
    "npm run build",   # trailing comma below
]
environment = "APP_VERSION={version}"

[publish]
push     = ["origin/main", "mirror/main"]
release  = true
workflow = "build.yml title=\"a b\""
command  = "ssh host 'cd /app && up'"
`

const YAML_FULL = `
# release config
version:
  preset: custom
  source: src/version.sh
  pattern: 'VERSION="{version}"'   # quoted

bump:
  level: patch
  commit: "release: {tag} # not a comment"

build:
  dir: web
  command:
    - npm ci
    - npm run build
  environment: APP_VERSION={version}

publish:
  push: [origin/main, mirror/main]
  release: true
  workflow: build.yml title="a b"
  command: ssh host 'cd /app && up'
`

func TestFormatsAgree(t *testing.T) {
	fromTOML, tomlError := Parse(TOML_FULL)
	if tomlError != nil {
		t.Fatal(tomlError)
	}

	fromYAML, yamlError := Parse(YAML_FULL)
	if yamlError != nil {
		t.Fatal(yamlError)
	}

	if !reflect.DeepEqual(fromTOML, fromYAML) {
		t.Fatalf("TOML and YAML differ:\n%+v\n%+v", fromTOML, fromYAML)
	}

	if fromTOML.Bump.Commit != "release: {tag} # not a comment" {
		t.Fatalf("commit = %q", fromTOML.Bump.Commit)
	}

	if fromTOML.Publish.Workflow.Items[0] != `build.yml title="a b"` {
		t.Fatalf("workflow = %q", fromTOML.Publish.Workflow.Items[0])
	}

	if !reflect.DeepEqual([]string(fromTOML.Build.Command), []string{"npm ci", "npm run build"}) {
		t.Fatalf("build.command = %q", fromTOML.Build.Command)
	}
}

func TestFormatErrors(t *testing.T) {
	cases := map[string]string{
		"toml unknown key":   "[bump]\nnope = \"x\"\n",
		"yaml unknown key":   "bump:\n  nope: x\n",
		"unknown section":    "[deploy]\nhost = \"x\"\n",
		"toml unquoted":      "[bump]\nlevel = patch\n\n[x\n",
		"yaml tab indent":    "bump:\n\tlevel: patch\n",
		"yaml anchor":        "bump:\n  level: &a patch\n",
		"yaml missing value": "bump:\n  level:\n",
		"wrong type":         "[bump]\nlevel = true\n",
		"duplicate key":      "[bump]\nlevel = \"a\"\nlevel = \"b\"\n",
		"neither":            "this is not a config",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, parseError := Parse(content)
			if parseError == nil {
				t.Fatalf("want an error for %q", content)
			}
		})
	}
}
