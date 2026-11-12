package version

import (
	"path/filepath"
	"strings"
)

// Format is a structured file format whose fields can be located.
type Format string

// The structured formats.
const (
	FORMAT_JSON Format = "json"
	FORMAT_TOML Format = "toml"
	FORMAT_YAML Format = "yaml"
	FORMAT_XML  Format = "xml"
)

// FormatOf returns the structured format of a file, or "" for any other file.
func FormatOf(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		{
			return FORMAT_JSON
		}
	case ".toml":
		{
			return FORMAT_TOML
		}
	case ".yaml", ".yml":
		{
			return FORMAT_YAML
		}
	case ".xml", ".csproj", ".fsproj", ".vbproj", ".props":
		{
			return FORMAT_XML
		}
	}

	return ""
}

// Span is the byte range of a value inside a file, without its quotes.
type Span struct {
	Start, End int
}

// Locate finds the value of a dotted field path, such as "project.version".
// It returns false when the field is missing or isn't a plain value.
func Locate(format Format, content string, path string) (Span, bool) {
	segments := strings.Split(path, ".")

	switch format {
	case FORMAT_JSON:
		{
			return locateJSON(content, segments)
		}
	case FORMAT_TOML:
		{
			return locateTOML(content, segments)
		}
	case FORMAT_YAML:
		{
			return locateYAML(content, segments)
		}
	case FORMAT_XML:
		{
			return locateXML(content, segments)
		}
	}

	return Span{}, false
}

func equalPath(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}
