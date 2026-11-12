package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestTOMLValues(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    map[string]any
	}{
		{"empty", "", map[string]any{}},
		{"comments only", "# a\n   # b\n", map[string]any{}},
		{"basic string", `[s]` + "\nk = \"v\"", map[string]any{"s": map[string]any{"k": "v"}}},
		{"literal string", "[s]\nk = 'C:\\path'", map[string]any{"s": map[string]any{"k": `C:\path`}}},
		{"no spaces", "[s]\nk=\"v\"", map[string]any{"s": map[string]any{"k": "v"}}},
		{"tabs", "[s]\n\tk\t=\t\"v\"", map[string]any{"s": map[string]any{"k": "v"}}},
		{"empty string", "[s]\nk = \"\"", map[string]any{"s": map[string]any{"k": ""}}},
		{"escapes", `[s]` + "\n" + `k = "a \"b\" c\\d\ne\tf"`, map[string]any{"s": map[string]any{"k": "a \"b\" c\\d\ne\tf"}}},
		{"hash in basic", "[s]\nk = \"a # b\" # real", map[string]any{"s": map[string]any{"k": "a # b"}}},
		{"hash in literal", "[s]\nk = 'a # b'", map[string]any{"s": map[string]any{"k": "a # b"}}},
		{"equals in value", "[s]\nk = \"A=1\"", map[string]any{"s": map[string]any{"k": "A=1"}}},
		{"booleans", "[s]\na = true\nb = false", map[string]any{"s": map[string]any{"a": true, "b": false}}},
		{"quoted key", "[s]\n\"k\" = \"v\"", map[string]any{"s": map[string]any{"k": "v"}}},
		{"section spaces", "[ s ]\nk = \"v\"", map[string]any{"s": map[string]any{"k": "v"}}},
		{"top-level key", "k = \"v\"", map[string]any{"k": "v"}},
		{"crlf", "[s]\r\nk = \"v\"\r\n", map[string]any{"s": map[string]any{"k": "v"}}},
		{"list", `[s]` + "\nk = [\"a\", 'b']", map[string]any{"s": map[string]any{"k": []any{"a", "b"}}}},
		{"list trailing comma", "[s]\nk = [\"a\",]", map[string]any{"s": map[string]any{"k": []any{"a"}}}},
		{"list comma in item", "[s]\nk = [\"a, b\", \"c\"]", map[string]any{"s": map[string]any{"k": []any{"a, b", "c"}}}},
		{"list bracket in item", "[s]\nk = [\"[x]\", \"y]\"]", map[string]any{"s": map[string]any{"k": []any{"[x]", "y]"}}}},
		{"empty list", "[s]\nk = []", map[string]any{"s": map[string]any{"k": []any(nil)}}},
		{
			"multi-line list",
			"[s]\nk = [\n  \"a\",  # one\n  \"b # not a comment\",\n  'c]'\n]\nafter = \"x\"",
			map[string]any{"s": map[string]any{"k": []any{"a", "b # not a comment", "c]"}, "after": "x"}},
		},
		{
			"two sections",
			"[a]\nk = \"1\"\n\n[b]\nk = \"2\"",
			map[string]any{"a": map[string]any{"k": "1"}, "b": map[string]any{"k": "2"}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, parseError := parseTOML(testCase.content)
			if parseError != nil {
				t.Fatalf("parseTOML: %v", parseError)
			}

			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("got  %#v\nwant %#v", got, testCase.want)
			}
		})
	}
}

func TestTOMLErrors(t *testing.T) {
	cases := map[string]string{
		"unquoted string":      "[s]\nk = v",
		"number":               "[s]\nk = 1",
		"text after string":    "[s]\nk = \"a\" \"b\"",
		"text after literal":   "[s]\nk = 'a' 'b'",
		"unclosed string":      "[s]\nk = \"a",
		"unclosed list":        "[s]\nk = [\"a\"",
		"unclosed list at eof": "[s]\nk = [\n\"a\",\n",
		"bad escape":           "[s]\nk = \"\\u0041\"",
		"trailing backslash":   "[s]\nk = \"a\\\"",
		"inline table":         "[s]\nk = {a = \"b\"}",
		"array of tables":      "[[s]]\nk = \"v\"",
		"dotted section":       "[a.b]\nk = \"v\"",
		"empty section":        "[]\nk = \"v\"",
		"unclosed section":     "[s\nk = \"v\"",
		"no equals":            "[s]\njust words",
		"no key":               "[s]\n= \"v\"",
		"duplicate key":        "[s]\nk = \"a\"\nk = \"b\"",
		"duplicate section":    "[s]\n[s]",
		"number in list":       "[s]\nk = [1, 2]",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, parseError := parseTOML(content)
			if parseError == nil {
				t.Fatalf("want an error for %q", content)
			}
		})
	}
}

func TestTOMLErrorLine(t *testing.T) {
	_, parseError := parseTOML("[s]\nk = \"v\"\n\nbad = value\n")
	if parseError == nil || !strings.HasPrefix(parseError.Error(), "line 4:") {
		t.Fatalf("want an error on line 4, got %v", parseError)
	}
}
