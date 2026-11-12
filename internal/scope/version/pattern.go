package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/diphyx/shipo/internal/text/semver"
)

// The capture groups a pattern can produce.
const (
	GROUP_VERSION = "version"
	GROUP_HASH    = "hash"
	GROUP_COUNTER = "counter"
	GROUP_DATE    = "date"
)

// patternKeywords maps each [version] pattern keyword to the regex it matches.
var patternKeywords = map[string]string{
	GROUP_VERSION: semver.PATTERN,
	GROUP_HASH:    `[0-9a-f]{4,40}`,
	GROUP_COUNTER: `\d+`,
	GROUP_DATE:    `\d{4}-\d{2}-\d{2}`,
	"*":           `[^\n]*?`,
	"_":           `[ \t]*`,
}

// Pattern is a compiled text, tag or regex pattern.
type Pattern struct {
	// Source is the pattern as written.
	Source string

	regex *regexp.Regexp
}

// IsRegex reports whether a pattern is written as /regex/.
func IsRegex(source string) bool {
	return len(source) >= 2 && strings.HasPrefix(source, "/") && strings.HasSuffix(source, "/")
}

// ParsePattern compiles a text pattern or a /regex/ pattern.
//
// In text mode everything is literal except keywords, a leading ^ and a
// trailing $, which are line anchors. In regex mode only keywords are
// replaced. The pattern must contain {version}.
func ParsePattern(source string) (*Pattern, error) {
	expression, buildError := buildExpression(source)
	if buildError != nil {
		return nil, buildError
	}

	return compile(source, "(?m)"+expression)
}

// ParseTag compiles a tag pattern that must match a whole tag name.
func ParseTag(source string) (*Pattern, error) {
	expression, buildError := buildExpression(source)
	if buildError != nil {
		return nil, buildError
	}

	return compile(source, `\A(?:`+expression+`)\z`)
}

func compile(source string, expression string) (*Pattern, error) {
	regex, compileError := regexp.Compile(expression)
	if compileError != nil {
		return nil, fmt.Errorf("pattern %q: %w", source, compileError)
	}

	return &Pattern{Source: source, regex: regex}, nil
}

func buildExpression(source string) (string, error) {
	isRegex := IsRegex(source)

	body := source
	if isRegex {
		body = source[1 : len(source)-1]
	}

	var builder strings.Builder

	if !isRegex && strings.HasPrefix(body, "^") {
		builder.WriteString("^")
		body = body[1:]
	}

	hasEnd := !isRegex && strings.HasSuffix(body, "$")
	if hasEnd {
		body = body[:len(body)-1]
	}

	seen := map[string]bool{}

	for index := 0; index < len(body); index++ {
		character := body[index]

		if character == '{' && index+1 < len(body) && body[index+1] == '{' {
			builder.WriteString(regexp.QuoteMeta("{"))
			index++

			continue
		}

		if character != '{' {
			if isRegex {
				builder.WriteByte(character)
			} else {
				builder.WriteString(regexp.QuoteMeta(string(character)))
			}

			continue
		}

		end := strings.IndexByte(body[index+1:], '}')
		if end < 0 {
			return "", fmt.Errorf("pattern %q: unclosed {", source)
		}

		name := body[index+1 : index+1+end]

		expression, isKnown := patternKeywords[name]
		if !isKnown {
			return "", fmt.Errorf("pattern %q: unknown keyword {%s}", source, name)
		}

		switch name {
		case "*", "_":
			{
				builder.WriteString(expression)
			}
		default:
			{
				if seen[name] {
					return "", fmt.Errorf("pattern %q: {%s} appears twice", source, name)
				}

				seen[name] = true
				builder.WriteString("(?P<" + name + ">" + expression + ")")
			}
		}

		index += end + 1
	}

	if hasEnd {
		builder.WriteString("$")
	}

	if !seen[GROUP_VERSION] {
		return "", fmt.Errorf("pattern %q: needs {version}", source)
	}

	return builder.String(), nil
}

// Match is one place a pattern matched.
type Match struct {
	// Version is the text captured by {version}.
	Version string
	// Counter is the number captured by {counter}, or -1.
	Counter int

	groups map[string][2]int
}

// FindAll returns every match in content.
func (pattern *Pattern) FindAll(content string) []Match {
	names := pattern.regex.SubexpNames()

	var matches []Match

	for _, location := range pattern.regex.FindAllStringSubmatchIndex(content, -1) {
		match := Match{Counter: -1, groups: map[string][2]int{}}

		for groupIndex, name := range names {
			start := location[2*groupIndex]
			if name == "" || start < 0 {
				continue
			}

			end := location[2*groupIndex+1]
			match.groups[name] = [2]int{start, end}

			switch name {
			case GROUP_VERSION:
				{
					match.Version = content[start:end]
				}
			case GROUP_COUNTER:
				{
					match.Counter, _ = strconv.Atoi(content[start:end])
				}
			}
		}

		matches = append(matches, match)
	}

	return matches
}

// MatchFull matches text with a pattern from ParseTag.
func (pattern *Pattern) MatchFull(text string) (Match, bool) {
	matches := pattern.FindAll(text)
	if len(matches) != 1 {
		return Match{}, false
	}

	return matches[0], true
}

// Fill is what a write puts into each group.
type Fill struct {
	Version string
	Hash    string
	Date    string
}

// Replace rewrites every match in content: {version}, {hash} and {date} get
// the fill values, and {counter} goes up by one. Everything else is kept.
func (pattern *Pattern) Replace(content string, matches []Match, fill Fill) string {
	type edit struct {
		start, end int
		text       string
	}

	var edits []edit

	for _, match := range matches {
		for name, span := range match.groups {
			text := ""

			switch name {
			case GROUP_VERSION:
				{
					text = fill.Version
				}
			case GROUP_HASH:
				{
					text = fill.Hash
				}
			case GROUP_DATE:
				{
					text = fill.Date
				}
			case GROUP_COUNTER:
				{
					text = strconv.Itoa(match.Counter + 1)
				}
			default:
				{
					continue
				}
			}

			if text == "" && name != GROUP_VERSION {
				continue
			}

			edits = append(edits, edit{start: span[0], end: span[1], text: text})
		}
	}

	return applyEdits(content, edits, func(item edit) (int, int, string) {
		return item.start, item.end, item.text
	})
}

// Render builds the text a pattern describes, for creating a git tag.
// Match keywords render as nothing.
func Render(source string, fill Fill) string {
	body := source
	if IsRegex(source) {
		body = source[1 : len(source)-1]
	}

	body = strings.TrimPrefix(body, "^")
	body = strings.TrimSuffix(body, "$")

	replacer := strings.NewReplacer(
		"{{", "{",
		"{version}", fill.Version,
		"{hash}", fill.Hash,
		"{date}", fill.Date,
		"{counter}", "",
		"{*}", "",
		"{_}", "",
	)

	return replacer.Replace(body)
}

func applyEdits[T any](content string, edits []T, span func(T) (int, int, string)) string {
	if len(edits) == 0 {
		return content
	}

	sorted := make([]T, len(edits))
	copy(sorted, edits)

	for outer := 1; outer < len(sorted); outer++ {
		for inner := outer; inner > 0; inner-- {
			previousStart, _, _ := span(sorted[inner-1])
			currentStart, _, _ := span(sorted[inner])

			if previousStart <= currentStart {
				break
			}

			sorted[inner-1], sorted[inner] = sorted[inner], sorted[inner-1]
		}
	}

	var builder strings.Builder

	cursor := 0

	for _, item := range sorted {
		start, end, text := span(item)
		builder.WriteString(content[cursor:start])
		builder.WriteString(text)
		cursor = end
	}

	builder.WriteString(content[cursor:])

	return builder.String()
}
