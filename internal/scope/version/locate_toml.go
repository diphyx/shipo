package version

import "strings"

// This file locates a key in TOML, line by line.

func locateTOML(content string, target []string) (Span, bool) {
	var table []string

	offset := 0
	multiline := ""

	for _, line := range strings.SplitAfter(content, "\n") {
		lineStart := offset
		offset += len(line)

		if multiline != "" {
			if strings.Contains(line, multiline) {
				multiline = ""
			}

			continue
		}

		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "[") {
			header := strings.TrimSpace(stripTOMLComment(trimmed))
			header = strings.TrimPrefix(strings.TrimSuffix(header, "]"), "[")
			header = strings.TrimPrefix(strings.TrimSuffix(header, "]"), "[")
			table = splitTOMLKey(header)

			continue
		}

		equals := strings.IndexByte(line, '=')
		if equals < 0 {
			continue
		}

		key := splitTOMLKey(strings.TrimSpace(line[:equals]))
		path := append(append([]string{}, table...), key...)

		valueStart := equals + 1
		for valueStart < len(line) && (line[valueStart] == ' ' || line[valueStart] == '\t') {
			valueStart++
		}

		rest := line[valueStart:]

		for _, delimiter := range []string{`"""`, `'''`} {
			if strings.HasPrefix(rest, delimiter) && !strings.Contains(rest[3:], delimiter) {
				multiline = delimiter
			}
		}

		if multiline != "" || !equalPath(path, target) {
			continue
		}

		if rest == "" || (rest[0] != '"' && rest[0] != '\'') {
			return Span{}, false
		}

		quote := rest[0]

		end := strings.IndexByte(rest[1:], quote)
		if end < 0 {
			return Span{}, false
		}

		start := lineStart + valueStart + 1

		return Span{Start: start, End: start + end}, true
	}

	return Span{}, false
}

func splitTOMLKey(key string) []string {
	var parts []string

	for _, part := range strings.Split(key, ".") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"'`)
		parts = append(parts, part)
	}

	return parts
}

func stripTOMLComment(line string) string {
	index := strings.IndexByte(line, '#')
	if index < 0 {
		return line
	}

	return line[:index]
}
