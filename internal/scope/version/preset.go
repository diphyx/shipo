// Package version reads and writes the version from presets, files and git
// tags.
package version

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The preset names.
const (
	PRESET_AUTO       = "auto"
	PRESET_CUSTOM     = "custom"
	PRESET_JAVASCRIPT = "javascript"
	PRESET_PYTHON     = "python"
	PRESET_RUST       = "rust"
	PRESET_PHP        = "php"
	PRESET_DART       = "dart"
	PRESET_JAVA       = "java"
	PRESET_CSHARP     = "csharp"
	PRESET_GO         = "go"
	PRESET_SWIFT      = "swift"
	PRESET_GIT        = "git"
)

// Preset is a predefined source and pattern.
type Preset struct {
	Name string
	// Detect is the file name that marks the stack in a folder.
	Detect string
	// Source is the default file or glob. Empty means git tags.
	Source string
	// Patterns are tried in order; the first that matches is used.
	Patterns []string
}

// IsGit reports whether the preset reads git tags.
func (preset Preset) IsGit() bool {
	return preset.Source == ""
}

// Presets lists the presets in detection order.
var Presets = []Preset{
	{Name: PRESET_JAVASCRIPT, Detect: "package.json", Source: "package.json", Patterns: []string{"version"}},
	{Name: PRESET_PYTHON, Detect: "pyproject.toml", Source: "pyproject.toml", Patterns: []string{"project.version", "tool.poetry.version"}},
	{Name: PRESET_RUST, Detect: "Cargo.toml", Source: "Cargo.toml", Patterns: []string{"package.version", "workspace.package.version"}},
	{Name: PRESET_PHP, Detect: "composer.json", Source: "composer.json", Patterns: []string{"version"}},
	{Name: PRESET_DART, Detect: "pubspec.yaml", Source: "pubspec.yaml", Patterns: []string{"version"}},
	{Name: PRESET_JAVA, Detect: "pom.xml", Source: "pom.xml", Patterns: []string{"project.version"}},
	{Name: PRESET_CSHARP, Detect: "*.csproj", Source: "*.csproj", Patterns: []string{"PropertyGroup.Version"}},
	{Name: PRESET_GO, Detect: "go.mod", Patterns: []string{"v{version}"}},
	{Name: PRESET_SWIFT, Detect: "Package.swift", Patterns: []string{"{version}"}},
	{Name: PRESET_GIT, Patterns: []string{"v{version}"}},
}

// Lookup returns a preset by name.
func Lookup(name string) (Preset, bool) {
	for _, preset := range Presets {
		if preset.Name == name {
			return preset, true
		}
	}

	return Preset{}, false
}

// Names lists every valid preset value.
func Names() []string {
	names := []string{PRESET_AUTO}

	for _, preset := range Presets {
		names = append(names, preset.Name)
	}

	return append(names, PRESET_CUSTOM)
}

// Detect finds the one stack in a folder.
func Detect(dir string) (Preset, error) {
	var found []Preset

	for _, preset := range Presets {
		if preset.Detect == "" {
			continue
		}

		matches, _ := filepath.Glob(filepath.Join(dir, preset.Detect))
		if len(matches) > 0 {
			found = append(found, preset)
		}
	}

	switch len(found) {
	case 0:
		{
			return Preset{}, fmt.Errorf("no stack detected in %s; set [version] preset", dir)
		}
	case 1:
		{
			return found[0], nil
		}
	}

	names := make([]string, len(found))
	for index, preset := range found {
		names[index] = preset.Name
	}

	return Preset{}, fmt.Errorf("several stacks detected (%s); set [version] preset", strings.Join(names, ", "))
}

// DetectFile finds the preset for a file name, such as packages/a/package.json.
func DetectFile(source string) (Preset, bool) {
	base := filepath.Base(source)

	for _, preset := range Presets {
		if preset.Source == "" {
			continue
		}

		isMatch, _ := filepath.Match(preset.Source, base)
		if isMatch {
			return preset, true
		}
	}

	return Preset{}, false
}

// Resolved is a [version] scope with its preset, files and patterns settled.
type Resolved struct {
	// Preset is the preset in use; "custom" when none applies.
	Preset string
	// IsGit is true when the version lives in git tags.
	IsGit bool
	// Source is the file or glob as written, relative to Dir.
	Source string
	// Files are the absolute files matched by Source. Empty for git.
	Files []string
	// Patterns are tried in order.
	Patterns []string
	// Dir is the .shipo folder.
	Dir string
}

// Settings is the [version] scope as written in .shipo.
type Settings struct {
	Preset  string
	Source  string
	Pattern string
}

// Resolve settles the preset, source files and patterns of a [version] scope.
func Resolve(settings Settings, dir string) (*Resolved, error) {
	preset, presetError := choosePreset(settings, dir)
	if presetError != nil {
		return nil, presetError
	}

	resolved := &Resolved{Preset: preset.Name, IsGit: preset.IsGit(), Source: preset.Source, Patterns: preset.Patterns, Dir: dir}

	if settings.Source != "" {
		if resolved.IsGit {
			return nil, fmt.Errorf("[version] preset %q reads git tags and takes no source", preset.Name)
		}

		resolved.Source = settings.Source
	}

	if settings.Pattern != "" {
		resolved.Patterns = []string{settings.Pattern}
	}

	if resolved.IsGit {
		_, tagError := ParseTag(resolved.Patterns[0])

		return resolved, tagError
	}

	files, globError := filepath.Glob(filepath.Join(dir, filepath.FromSlash(resolved.Source)))
	if globError != nil {
		return nil, fmt.Errorf("[version] source %q: %w", resolved.Source, globError)
	}

	var regular []string

	for _, file := range files {
		info, statError := os.Stat(file)
		if statError == nil && !info.IsDir() {
			regular = append(regular, file)
		}
	}

	if len(regular) == 0 {
		return nil, fmt.Errorf("[version] source %q matches no file", resolved.Source)
	}

	sort.Strings(regular)
	resolved.Files = regular

	return resolved, nil
}

func choosePreset(settings Settings, dir string) (Preset, error) {
	name := settings.Preset
	if name == "" {
		name = PRESET_AUTO
	}

	switch name {
	case PRESET_CUSTOM:
		{
			if settings.Source == "" || settings.Pattern == "" {
				return Preset{}, errors.New("[version] preset \"custom\" needs source and pattern")
			}

			return Preset{Name: PRESET_CUSTOM, Source: settings.Source, Patterns: []string{settings.Pattern}}, nil
		}
	case PRESET_AUTO:
		{
			return chooseAuto(settings, dir)
		}
	}

	preset, isKnown := Lookup(name)
	if !isKnown {
		return Preset{}, fmt.Errorf("[version] unknown preset %q: want %s", name, strings.Join(Names(), ", "))
	}

	return preset, nil
}

func chooseAuto(settings Settings, dir string) (Preset, error) {
	switch {
	case settings.Source == "" && settings.Pattern == "":
		{
			return Detect(dir)
		}
	case settings.Source != "" && settings.Pattern != "":
		{
			return Preset{Name: PRESET_CUSTOM, Source: settings.Source, Patterns: []string{settings.Pattern}}, nil
		}
	case settings.Source != "":
		{
			preset, isFound := DetectFile(settings.Source)
			if !isFound {
				return Preset{}, fmt.Errorf("[version] no preset for %q; set a pattern", settings.Source)
			}

			return preset, nil
		}
	}

	return Preset{}, errors.New("[version] pattern needs a source, or a git preset")
}
