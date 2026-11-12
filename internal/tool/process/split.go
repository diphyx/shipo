// Package process splits a command line into arguments and spawns it
// directly, without a shell.
package process

import (
	"errors"
	"fmt"
	"strings"
)

// ErrEmpty is returned when a command has no arguments.
var ErrEmpty = errors.New("empty command")

// Split breaks a command line into arguments, the same way on every OS.
//
// Whitespace separates arguments. Single quotes keep everything literal.
// Double quotes keep whitespace; inside them, a backslash escapes " and \.
// Outside quotes, a backslash escapes the next character.
func Split(line string) ([]string, error) {
	var (
		arguments []string
		current   strings.Builder
		isInWord  bool
		quote     byte
	)

	for index := 0; index < len(line); index++ {
		character := line[index]

		switch quote {
		case '\'':
			{
				if character == '\'' {
					quote = 0
				} else {
					current.WriteByte(character)
				}
			}
		case '"':
			{
				isEscape := character == '\\' && index+1 < len(line) && (line[index+1] == '"' || line[index+1] == '\\')

				switch {
				case isEscape:
					{
						index++
						current.WriteByte(line[index])
					}
				case character == '"':
					{
						quote = 0
					}
				default:
					{
						current.WriteByte(character)
					}
				}
			}
		default:
			{
				switch character {
				case ' ', '\t', '\n', '\r':
					{
						if isInWord {
							arguments = append(arguments, current.String())
							current.Reset()
							isInWord = false
						}
					}
				case '\'', '"':
					{
						quote = character
						isInWord = true
					}
				case '\\':
					{
						isInWord = true

						if index+1 < len(line) {
							index++
							current.WriteByte(line[index])
						}
					}
				default:
					{
						isInWord = true
						current.WriteByte(character)
					}
				}
			}
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("unclosed %c quote in %q", quote, line)
	}

	if isInWord {
		arguments = append(arguments, current.String())
	}

	if len(arguments) == 0 {
		return nil, ErrEmpty
	}

	return arguments, nil
}
