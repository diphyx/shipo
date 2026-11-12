package version

import "strings"

// This file locates an element in XML, such as pom.xml or a .csproj.

func locateXML(content string, target []string) (Span, bool) {
	var stack []string

	textStart := -1
	index := 0

	for index < len(content) {
		if content[index] != '<' {
			index++

			continue
		}

		switch {
		case strings.HasPrefix(content[index:], "<!--"):
			{
				end := strings.Index(content[index:], "-->")
				if end < 0 {
					return Span{}, false
				}

				index += end + 3

				continue
			}
		case strings.HasPrefix(content[index:], "<![CDATA["):
			{
				end := strings.Index(content[index:], "]]>")
				if end < 0 {
					return Span{}, false
				}

				index += end + 3

				continue
			}
		case strings.HasPrefix(content[index:], "<?"), strings.HasPrefix(content[index:], "<!"):
			{
				end := strings.IndexByte(content[index:], '>')
				if end < 0 {
					return Span{}, false
				}

				index += end + 1

				continue
			}
		}

		end := strings.IndexByte(content[index:], '>')
		if end < 0 {
			return Span{}, false
		}

		tag := content[index+1 : index+end]
		tagEnd := index + end + 1

		if strings.HasPrefix(tag, "/") {
			if textStart >= 0 && matchesXMLPath(stack, target) {
				start, finish := trimSpan(content, textStart, index)
				if finish > start {
					return Span{Start: start, End: finish}, true
				}
			}

			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

			textStart = -1
		} else if strings.HasSuffix(tag, "/") {
			textStart = -1
		} else {
			name := strings.Fields(tag)[0]

			colon := strings.LastIndexByte(name, ':')
			if colon >= 0 {
				name = name[colon+1:]
			}

			stack = append(stack, name)
			textStart = tagEnd
		}

		index = tagEnd
	}

	return Span{}, false
}

func matchesXMLPath(stack []string, target []string) bool {
	if equalPath(stack, target) {
		return true
	}

	return len(stack) > 1 && equalPath(stack[1:], target)
}

func trimSpan(content string, start int, end int) (int, int) {
	for start < end && strings.ContainsRune(" \t\r\n", rune(content[start])) {
		start++
	}

	for end > start && strings.ContainsRune(" \t\r\n", rune(content[end-1])) {
		end--
	}

	return start, end
}
