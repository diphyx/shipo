package version

import (
	"errors"
	"strings"
)

// This file locates a field in JSON with a small scanner.

type jsonScanner struct {
	content string
	index   int
	target  []string
	found   Span
	isFound bool
}

func locateJSON(content string, target []string) (Span, bool) {
	scanner := &jsonScanner{content: content, target: target}

	scanError := scanner.value(nil)
	if scanError != nil {
		return Span{}, false
	}

	return scanner.found, scanner.isFound
}

func (scanner *jsonScanner) skipSpace() {
	for scanner.index < len(scanner.content) {
		switch scanner.content[scanner.index] {
		case ' ', '\t', '\n', '\r':
			{
				scanner.index++
			}
		default:
			{
				return
			}
		}
	}
}

func (scanner *jsonScanner) value(path []string) error {
	scanner.skipSpace()

	if scanner.index >= len(scanner.content) {
		return errors.New("unexpected end of JSON")
	}

	switch scanner.content[scanner.index] {
	case '{':
		{
			return scanner.object(path)
		}
	case '[':
		{
			return scanner.array(path)
		}
	case '"':
		{
			start, end, stringError := scanner.text()
			if stringError != nil {
				return stringError
			}

			if !scanner.isFound && equalPath(path, scanner.target) {
				scanner.found = Span{Start: start, End: end}
				scanner.isFound = true
			}

			return nil
		}
	}

	for scanner.index < len(scanner.content) && !strings.ContainsRune(",}] \t\n\r", rune(scanner.content[scanner.index])) {
		scanner.index++
	}

	return nil
}

func (scanner *jsonScanner) text() (int, int, error) {
	scanner.index++
	start := scanner.index

	for scanner.index < len(scanner.content) {
		switch scanner.content[scanner.index] {
		case '\\':
			{
				scanner.index += 2
			}
		case '"':
			{
				end := scanner.index
				scanner.index++

				return start, end, nil
			}
		default:
			{
				scanner.index++
			}
		}
	}

	return 0, 0, errors.New("unclosed JSON string")
}

func (scanner *jsonScanner) object(path []string) error {
	scanner.index++

	for {
		scanner.skipSpace()

		if scanner.index >= len(scanner.content) {
			return errors.New("unclosed JSON object")
		}

		if scanner.content[scanner.index] == '}' {
			scanner.index++

			return nil
		}

		if scanner.content[scanner.index] == ',' {
			scanner.index++

			continue
		}

		if scanner.content[scanner.index] != '"' {
			return errors.New("JSON key expected")
		}

		start, end, keyError := scanner.text()
		if keyError != nil {
			return keyError
		}

		key := scanner.content[start:end]

		scanner.skipSpace()

		if scanner.index >= len(scanner.content) || scanner.content[scanner.index] != ':' {
			return errors.New("JSON colon expected")
		}

		scanner.index++

		childPath := append(append([]string{}, path...), key)

		valueError := scanner.value(childPath)
		if valueError != nil {
			return valueError
		}
	}
}

func (scanner *jsonScanner) array(path []string) error {
	scanner.index++

	for {
		scanner.skipSpace()

		if scanner.index >= len(scanner.content) {
			return errors.New("unclosed JSON array")
		}

		switch scanner.content[scanner.index] {
		case ']':
			{
				scanner.index++

				return nil
			}
		case ',':
			{
				scanner.index++
			}
		default:
			{
				valueError := scanner.value(append(append([]string{}, path...), "[]"))
				if valueError != nil {
					return valueError
				}
			}
		}
	}
}
