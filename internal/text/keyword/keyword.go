// Package keyword expands the shared {name} values used in commit messages,
// build and publish commands.
package keyword

import (
	"strings"
)

// Values maps a keyword name, without braces, to its value.
type Values map[string]string

// Expand replaces every known {name} in text with its value.
//
// An unknown {name} is left as written, and {{ gives a literal {.
func Expand(text string, values Values) string {
	var builder strings.Builder

	builder.Grow(len(text))

	for index := 0; index < len(text); index++ {
		character := text[index]

		if character != '{' {
			builder.WriteByte(character)

			continue
		}

		if index+1 < len(text) && text[index+1] == '{' {
			builder.WriteByte('{')
			index++

			continue
		}

		end := strings.IndexByte(text[index+1:], '}')
		if end < 0 {
			builder.WriteString(text[index:])

			break
		}

		name := text[index+1 : index+1+end]
		value, isKnown := values[name]

		if isKnown {
			builder.WriteString(value)
		} else {
			builder.WriteString(text[index : index+end+2])
		}

		index += end + 1
	}

	return builder.String()
}

// ExpandAll expands every entry of a list.
func ExpandAll(texts []string, values Values) []string {
	expanded := make([]string, len(texts))

	for index, text := range texts {
		expanded[index] = Expand(text, values)
	}

	return expanded
}
