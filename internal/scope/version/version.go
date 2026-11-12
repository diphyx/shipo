package version

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/diphyx/shipo/internal/text/semver"
)

// Location is where one file holds the version.
type Location struct {
	File string

	content string
	spans   []Span
	matches []Match
	text    *Pattern
}

// Current is the version read from every file of a Resolved scope.
type Current struct {
	Version   semver.Version
	Locations []Location

	resolved *Resolved
}

// Read reads the current version. For git sources, tags must hold the
// repository's tags; for file sources it is ignored.
func Read(resolved *Resolved, tags []string) (*Current, error) {
	if resolved.IsGit {
		return readTags(resolved, tags)
	}

	current := &Current{resolved: resolved}
	found := ""

	for _, file := range resolved.Files {
		location, readError := readFile(file, resolved.Patterns)
		if readError != nil {
			return nil, readError
		}

		for _, text := range location.versions() {
			if found == "" {
				found = text
			} else if text != found {
				return nil, fmt.Errorf("versions differ: %s in %s, but %s elsewhere", text, relative(resolved.Dir, file), found)
			}
		}

		current.Locations = append(current.Locations, location)
	}

	version, parseError := semver.Parse(found)
	if parseError != nil {
		return nil, parseError
	}

	current.Version = version

	return current, nil
}

func readTags(resolved *Resolved, tags []string) (*Current, error) {
	pattern, compileError := ParseTag(resolved.Patterns[0])
	if compileError != nil {
		return nil, compileError
	}

	current := &Current{Version: semver.ZERO, resolved: resolved}
	isFound := false

	for _, tag := range tags {
		match, isMatch := pattern.MatchFull(tag)
		if !isMatch {
			continue
		}

		version, parseError := semver.Parse(match.Version)
		if parseError != nil {
			continue
		}

		if !isFound || version.Compare(current.Version) > 0 {
			current.Version = version
			isFound = true
		}
	}

	return current, nil
}

func readFile(file string, patterns []string) (Location, error) {
	data, readError := os.ReadFile(file)
	if readError != nil {
		return Location{}, readError
	}

	content := string(data)
	format := FormatOf(file)

	for _, source := range patterns {
		location := Location{File: file, content: content}

		if format != "" && !IsRegex(source) && !strings.Contains(source, "{") {
			span, isFound := Locate(format, content, source)
			if !isFound {
				continue
			}

			location.spans = []Span{span}

			return location, nil
		}

		pattern, compileError := ParsePattern(source)
		if compileError != nil {
			return Location{}, compileError
		}

		matches := pattern.FindAll(content)
		if len(matches) == 0 {
			continue
		}

		location.text = pattern
		location.matches = matches

		return location, nil
	}

	if isDynamic(file, content) {
		return Location{}, fmt.Errorf("%s: the version is dynamic; set [version] to the file that holds it", filepath.Base(file))
	}

	return Location{}, fmt.Errorf("%s: no version found for %s", filepath.Base(file), strings.Join(patterns, " or "))
}

func isDynamic(file string, content string) bool {
	return filepath.Base(file) == "pyproject.toml" && strings.Contains(content, "dynamic") && strings.Contains(content, `"version"`)
}

func (location Location) versions() []string {
	var texts []string

	for _, span := range location.spans {
		texts = append(texts, location.content[span.Start:span.End])
	}

	for _, match := range location.matches {
		texts = append(texts, match.Version)
	}

	return texts
}

// render returns the file content with the new version written in.
func (location Location) render(fill Fill) string {
	if location.text != nil {
		return location.text.Replace(location.content, location.matches, fill)
	}

	return applyEdits(location.content, location.spans, func(span Span) (int, int, string) {
		return span.Start, span.End, fill.Version
	})
}

// Write writes a new version into every file, all or none. It returns the
// files it changed.
func (current *Current) Write(fill Fill) ([]string, error) {
	type pending struct {
		file     string
		original []byte
		updated  []byte
		mode     os.FileMode
	}

	var queue []pending

	for _, location := range current.Locations {
		updated := []byte(location.render(fill))
		original := []byte(location.content)

		if bytes.Equal(updated, original) {
			continue
		}

		info, statError := os.Stat(location.File)
		if statError != nil {
			return nil, statError
		}

		queue = append(queue, pending{file: location.File, original: original, updated: updated, mode: info.Mode().Perm()})
	}

	var written []pending

	for _, item := range queue {
		writeError := os.WriteFile(item.file, item.updated, item.mode)
		if writeError == nil {
			written = append(written, item)

			continue
		}

		var restoreErrors []error

		for _, done := range written {
			restoreErrors = append(restoreErrors, os.WriteFile(done.file, done.original, done.mode))
		}

		return nil, errors.Join(append([]error{fmt.Errorf("write %s: %w", item.file, writeError)}, restoreErrors...)...)
	}

	files := make([]string, len(written))
	for index, item := range written {
		files[index] = item.file
	}

	return files, nil
}

// Files lists the files that hold the version.
func (current *Current) Files() []string {
	files := make([]string, len(current.Locations))
	for index, location := range current.Locations {
		files[index] = location.File
	}

	return files
}

// Resolved returns the scope the version was read from.
func (current *Current) Resolved() *Resolved {
	return current.resolved
}

func relative(dir string, path string) string {
	relativePath, relativeError := filepath.Rel(dir, path)
	if relativeError != nil {
		return path
	}

	return relativePath
}
