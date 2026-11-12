// Package config finds the nearest .shipo and loads its scopes.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FILE_NAME is the name of the config file.
const FILE_NAME = ".shipo"

// ErrNotFound is returned when no .shipo exists in the folder or above it.
var ErrNotFound = errors.New("no .shipo found in this folder or above")

// Config is a loaded .shipo.
type Config struct {
	// Path is the .shipo file. Dir is its folder: the target.
	Path string
	Dir  string

	Version Version
	Bump    Bump
	Build   Build
	Publish Publish
}

// Version is the [version] scope.
type Version struct {
	Preset  string
	Source  string
	Pattern string
}

// Bump is the [bump] scope.
type Bump struct {
	Level  string
	Commit string
}

// Build is the [build] scope.
type Build struct {
	Dir         string
	Command     List
	Environment List
}

// Publish is the [publish] scope.
type Publish struct {
	Push     Switch
	Release  Switch
	Workflow Switch
	Command  List
}

// The defaults for keys that are not set.
const (
	DEFAULT_PRESET = "auto"
	DEFAULT_LEVEL  = "ask"
	DEFAULT_COMMIT = "chore: release v{version}"
	DEFAULT_PUSH   = "origin/main"
)

// Find returns the nearest .shipo, from dir upwards.
func Find(dir string) (string, error) {
	current, absoluteError := filepath.Abs(dir)
	if absoluteError != nil {
		return "", absoluteError
	}

	for {
		candidate := filepath.Join(current, FILE_NAME)

		info, statError := os.Stat(candidate)
		if statError == nil && !info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrNotFound
		}

		current = parent
	}
}

// Load reads and validates a .shipo file.
func Load(path string) (*Config, error) {
	data, readError := os.ReadFile(path)
	if readError != nil {
		return nil, readError
	}

	config, parseError := Parse(string(data))
	if parseError != nil {
		return nil, fmt.Errorf("%s: %w", path, parseError)
	}

	absolute, absoluteError := filepath.Abs(path)
	if absoluteError != nil {
		return nil, absoluteError
	}

	config.Path = absolute
	config.Dir = filepath.Dir(absolute)

	return config, nil
}

// Parse decodes .shipo content, written in TOML or YAML, applies defaults and
// validates it. TOML is tried first, then YAML.
func Parse(content string) (*Config, error) {
	root, tomlError := parseTOML(content)
	if tomlError != nil {
		parsed, yamlError := parseYAML(content)
		if yamlError != nil {
			return nil, fmt.Errorf("not valid TOML (%v) or YAML (%v)", tomlError, yamlError)
		}

		root = parsed
	}

	config, decodeError := decode(root)
	if decodeError != nil {
		return nil, decodeError
	}

	config.applyDefaults()

	validateError := config.validate()
	if validateError != nil {
		return nil, validateError
	}

	return config, nil
}

func (config *Config) applyDefaults() {
	if config.Version.Preset == "" {
		config.Version.Preset = DEFAULT_PRESET
	}

	if config.Bump.Level == "" {
		config.Bump.Level = DEFAULT_LEVEL
	}

	if config.Bump.Commit == "" {
		config.Bump.Commit = DEFAULT_COMMIT
	}

	if !config.Publish.Push.IsSet {
		config.Publish.Push = Switch{IsSet: true, Items: []string{DEFAULT_PUSH}}
	}
}

func (config *Config) validate() error {
	push := config.Publish.Push
	if push.IsOn {
		return errors.New("publish.push: want \"remote/branch\", a list, or false")
	}

	for _, target := range push.Items {
		_, _, splitError := SplitPush(target)
		if splitError != nil {
			return fmt.Errorf("publish.push: %w", splitError)
		}
	}

	workflow := config.Publish.Workflow
	if workflow.IsOn {
		return errors.New("publish.workflow: want a workflow file, a list, or false")
	}

	for _, entry := range config.Build.Environment {
		if !strings.Contains(entry, "=") || strings.HasPrefix(entry, "=") {
			return fmt.Errorf("build.environment: %q is not NAME=value", entry)
		}
	}

	if config.Build.Dir != "" && len(config.Build.Command) == 0 {
		return errors.New("build.command is required when [build] is set")
	}

	return nil
}

// SplitPush splits "remote/branch" at the first slash.
func SplitPush(target string) (remote string, branch string, splitError error) {
	index := strings.IndexByte(target, '/')
	if index <= 0 || index == len(target)-1 {
		return "", "", fmt.Errorf("%q is not remote/branch", target)
	}

	return target[:index], target[index+1:], nil
}

// Rel returns path relative to the target folder, for display.
func (config *Config) Rel(path string) string {
	relative, relativeError := filepath.Rel(config.Dir, path)
	if relativeError != nil {
		return path
	}

	return relative
}
