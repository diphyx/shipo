package config

import (
	"errors"
	"fmt"
	"strings"
)

type yamlLine struct {
	number int
	indent int
	text   string
}

// parseYAML reads the small part of YAML that .shipo uses: nested keys by
// indentation, plain or quoted strings, booleans, "- item" lists, [a, b]
// lists, and # comments.
func parseYAML(content string) (map[string]any, error) {
	var lines []yamlLine

	for index, raw := range strings.Split(content, "\n") {
		leading := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
		if strings.Contains(leading, "\t") {
			return nil, fmt.Errorf("line %d: use spaces, not tabs, to indent", index+1)
		}

		text := strings.TrimRight(stripComment(raw, true), " \r")
		trimmed := strings.TrimLeft(text, " ")

		if trimmed == "" || trimmed == "---" {
			continue
		}

		lines = append(lines, yamlLine{number: index + 1, indent: len(text) - len(trimmed), text: trimmed})
	}

	if len(lines) == 0 {
		return map[string]any{}, nil
	}

	mapping, next, mappingError := yamlMapping(lines, 0, lines[0].indent)
	if mappingError != nil {
		return nil, mappingError
	}

	if next < len(lines) {
		return nil, fmt.Errorf("line %d: unexpected indentation", lines[next].number)
	}

	return mapping, nil
}

func yamlMapping(lines []yamlLine, start int, indent int) (map[string]any, int, error) {
	mapping := map[string]any{}
	index := start

	for index < len(lines) && lines[index].indent == indent {
		line := lines[index]

		colon := strings.Index(line.text, ":")
		if colon <= 0 || (colon+1 < len(line.text) && line.text[colon+1] != ' ') {
			return nil, 0, fmt.Errorf("line %d: want key: value", line.number)
		}

		key := strings.Trim(line.text[:colon], `"'`)
		rest := strings.TrimSpace(line.text[colon+1:])

		_, isDuplicate := mapping[key]
		if isDuplicate {
			return nil, 0, fmt.Errorf("line %d: %s appears twice", line.number, key)
		}

		index++

		if rest != "" {
			value, valueError := yamlScalar(rest)
			if valueError != nil {
				return nil, 0, fmt.Errorf("line %d: %s: %w", line.number, key, valueError)
			}

			mapping[key] = value

			continue
		}

		if index >= len(lines) {
			return nil, 0, fmt.Errorf("line %d: %s has no value", line.number, key)
		}

		child := lines[index]

		switch {
		case strings.HasPrefix(child.text, "- ") && child.indent >= indent:
			{
				items, next, listError := yamlList(lines, index, child.indent)
				if listError != nil {
					return nil, 0, listError
				}

				mapping[key] = items
				index = next
			}
		case child.indent > indent:
			{
				nested, next, nestedError := yamlMapping(lines, index, child.indent)
				if nestedError != nil {
					return nil, 0, nestedError
				}

				mapping[key] = nested
				index = next
			}
		default:
			{
				return nil, 0, fmt.Errorf("line %d: %s has no value", line.number, key)
			}
		}
	}

	return mapping, index, nil
}

func yamlList(lines []yamlLine, start int, indent int) ([]any, int, error) {
	var items []any

	index := start

	for index < len(lines) && lines[index].indent == indent && strings.HasPrefix(lines[index].text, "- ") {
		value, valueError := yamlItem(strings.TrimSpace(lines[index].text[2:]))
		if valueError != nil {
			return nil, 0, fmt.Errorf("line %d: %w", lines[index].number, valueError)
		}

		items = append(items, value)
		index++
	}

	return items, index, nil
}

func yamlScalar(text string) (any, error) {
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

			return listValue(text[1:len(text)-1], yamlItem)
		}
	}

	return yamlItem(text)
}

func yamlItem(text string) (any, error) {
	if text == "" {
		return nil, errors.New("empty value")
	}

	if text[0] == '"' || text[0] == '\'' {
		return quoted(text)
	}

	if strings.ContainsAny(text[:1], "{&*|>!%@`") {
		return nil, fmt.Errorf("unsupported value %s", text)
	}

	if strings.Contains(text, ": ") || strings.HasSuffix(text, ":") {
		return nil, fmt.Errorf("unsupported nested map %s; quote it if it is text", text)
	}

	return text, nil
}
