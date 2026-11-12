package version

import "strings"

// This file locates a key in YAML by indentation.

func locateYAML(content string, target []string) (Span, bool) {
	type level struct {
		indent int
		key    string
	}

	var stack []level

	offset := 0

	for _, line := range strings.SplitAfter(content, "\n") {
		lineStart := offset
		offset += len(line)

		body := strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimLeft(body, " ")
		indent := len(body) - len(trimmed)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "- ") {
			continue
		}

		colon := strings.Index(trimmed, ":")
		if colon <= 0 || (colon+1 < len(trimmed) && trimmed[colon+1] != ' ') {
			continue
		}

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		key := strings.Trim(trimmed[:colon], `"'`)
		stack = append(stack, level{indent: indent, key: key})

		path := make([]string, len(stack))
		for index, item := range stack {
			path[index] = item.key
		}

		if !equalPath(path, target) {
			continue
		}

		valueStart := indent + colon + 1
		for valueStart < len(body) && body[valueStart] == ' ' {
			valueStart++
		}

		valueEnd := len(body)

		if valueStart < len(body) && (body[valueStart] == '"' || body[valueStart] == '\'') {
			closing := strings.IndexByte(body[valueStart+1:], body[valueStart])
			if closing < 0 {
				return Span{}, false
			}

			valueStart++
			valueEnd = valueStart + closing
		} else {
			comment := strings.Index(body[valueStart:], " #")
			if comment >= 0 {
				valueEnd = valueStart + comment
			}

			for valueEnd > valueStart && body[valueEnd-1] == ' ' {
				valueEnd--
			}
		}

		if valueEnd <= valueStart {
			return Span{}, false
		}

		return Span{Start: lineStart + valueStart, End: lineStart + valueEnd}, true
	}

	return Span{}, false
}
