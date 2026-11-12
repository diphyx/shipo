package config

import (
	"errors"
	"fmt"
	"strings"
)

// parseTOML reads the small part of TOML that .shipo uses: [section] headers,
// key = value lines, strings, booleans, lists of strings, and # comments.
func parseTOML(content string) (map[string]any, error) {
	root := map[string]any{}
	section := root

	lines := strings.Split(content, "\n")

	for index := 0; index < len(lines); index++ {
		number := index + 1
		line := strings.TrimSpace(stripComment(lines[index], false))

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") || strings.HasPrefix(line, "[[") {
				return nil, fmt.Errorf("line %d: bad section %q", number, line)
			}

			name := strings.TrimSpace(line[1 : len(line)-1])
			if name == "" || strings.ContainsAny(name, ". ") {
				return nil, fmt.Errorf("line %d: bad section %q", number, line)
			}

			_, isDuplicate := root[name]
			if isDuplicate {
				return nil, fmt.Errorf("line %d: section [%s] appears twice", number, name)
			}

			section = map[string]any{}
			root[name] = section

			continue
		}

		equals := strings.IndexByte(line, '=')
		if equals <= 0 {
			return nil, fmt.Errorf("line %d: want key = value", number)
		}

		key := strings.Trim(strings.TrimSpace(line[:equals]), `"'`)
		text := strings.TrimSpace(line[equals+1:])

		if strings.HasPrefix(text, "[") {
			depth := bracketDepth(text)

			var builder strings.Builder

			builder.WriteString(text)

			for depth > 0 && index+1 < len(lines) {
				index++
				next := strings.TrimSpace(stripComment(lines[index], false))
				depth += bracketDepth(next)

				builder.WriteByte(' ')
				builder.WriteString(next)
			}

			text = builder.String()
		}

		value, valueError := tomlValue(text)
		if valueError != nil {
			return nil, fmt.Errorf("line %d: %s: %w", number, key, valueError)
		}

		_, isDuplicate := section[key]
		if isDuplicate {
			return nil, fmt.Errorf("line %d: %s appears twice", number, key)
		}

		section[key] = value
	}

	return root, nil
}

func tomlValue(text string) (any, error) {
	switch {
	case text == "true":
		{
			return true, nil
		}
	case text == "false":
		{
			return false, nil
		}
	case strings.HasPrefix(text, "["):
		{
			if !strings.HasSuffix(text, "]") {
				return nil, errors.New("unclosed list")
			}

			return listValue(text[1:len(text)-1], func(item string) (any, error) {
				return quoted(item)
			})
		}
	}

	return quoted(text)
}

// listValue splits the inside of [ … ] at commas outside quotes.
func listValue(inside string, item func(string) (any, error)) ([]any, error) {
	var items []any

	for _, part := range splitOutsideQuotes(inside, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		value, itemError := item(part)
		if itemError != nil {
			return nil, itemError
		}

		items = append(items, value)
	}

	return items, nil
}

// quoted reads a "basic" or 'literal' string that fills the whole text.
func quoted(text string) (string, error) {
	if len(text) < 2 || (text[0] != '"' && text[0] != '\'') || text[len(text)-1] != text[0] {
		return "", fmt.Errorf("want a quoted string, true, false or a list, got %s", text)
	}

	body := text[1 : len(text)-1]
	if text[0] == '\'' {
		if strings.IndexByte(body, '\'') >= 0 {
			return "", fmt.Errorf("unexpected text after string in %s", text)
		}

		return body, nil
	}

	var builder strings.Builder

	for index := 0; index < len(body); index++ {
		if body[index] == '"' {
			return "", fmt.Errorf("unexpected text after string in %s", text)
		}

		if body[index] != '\\' {
			builder.WriteByte(body[index])

			continue
		}

		if index+1 >= len(body) {
			return "", errors.New("string ends with \\")
		}

		index++

		switch body[index] {
		case '"', '\\':
			{
				builder.WriteByte(body[index])
			}
		case 'n':
			{
				builder.WriteByte('\n')
			}
		case 't':
			{
				builder.WriteByte('\t')
			}
		default:
			{
				return "", fmt.Errorf("unsupported escape \\%c", body[index])
			}
		}
	}

	return builder.String(), nil
}

// stripComment removes a # comment that is outside quotes. YAML needs a space
// or the line start before #; TOML doesn't.
func stripComment(line string, isSpaceRequired bool) string {
	quote := byte(0)

	for index := 0; index < len(line); index++ {
		character := line[index]

		switch {
		case quote != 0 && character == '\\' && quote == '"':
			{
				index++
			}
		case quote != 0 && character == quote:
			{
				quote = 0
			}
		case quote == 0 && (character == '"' || character == '\''):
			{
				quote = character
			}
		case quote == 0 && character == '#' && (!isSpaceRequired || index == 0 || line[index-1] == ' '):
			{
				return line[:index]
			}
		}
	}

	return line
}

// bracketDepth returns how many [ minus ] a line has, outside quotes. Strings
// never span lines, so lines can be counted one by one.
func bracketDepth(line string) int {
	depth := 0

	for _, part := range splitOutsideQuotes(line, 0) {
		depth += strings.Count(part, "[") - strings.Count(part, "]")
	}

	return depth
}

// splitOutsideQuotes splits text at separator, ignoring separators inside
// quotes. A zero separator returns the unquoted pieces only.
func splitOutsideQuotes(text string, separator byte) []string {
	var (
		parts   []string
		current strings.Builder
		quote   byte
	)

	for index := 0; index < len(text); index++ {
		character := text[index]

		switch {
		case quote != 0:
			{
				if separator != 0 {
					current.WriteByte(character)
				}

				if character == '\\' && quote == '"' && index+1 < len(text) {
					index++

					if separator != 0 {
						current.WriteByte(text[index])
					}
				} else if character == quote {
					quote = 0
				}
			}
		case character == '"' || character == '\'':
			{
				quote = character

				if separator != 0 {
					current.WriteByte(character)
				}
			}
		case separator != 0 && character == separator:
			{
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			{
				current.WriteByte(character)
			}
		}
	}

	return append(parts, current.String())
}
