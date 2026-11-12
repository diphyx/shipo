package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestYAMLValues(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    map[string]any
	}{
		{"empty", "", map[string]any{}},
		{"comments only", "# a\n  # b\n", map[string]any{}},
		{"document marker", "---\ns:\n  k: v\n", map[string]any{"s": map[string]any{"k": "v"}}},
		{"plain", "s:\n  k: v", map[string]any{"s": map[string]any{"k": "v"}}},
		{"plain with spaces", "s:\n  k: npm run build", map[string]any{"s": map[string]any{"k": "npm run build"}}},
		{"double quoted", "s:\n  k: \"a \\\"b\\\"\"", map[string]any{"s": map[string]any{"k": `a "b"`}}},
		{"single quoted", "s:\n  k: 'C:\\path'", map[string]any{"s": map[string]any{"k": `C:\path`}}},
		{"empty quoted", "s:\n  k: \"\"", map[string]any{"s": map[string]any{"k": ""}}},
		{"colon in value", "s:\n  k: https://example.com:8080/x", map[string]any{"s": map[string]any{"k": "https://example.com:8080/x"}}},
		{"colon in quotes", "s:\n  k: \"a: b\"", map[string]any{"s": map[string]any{"k": "a: b"}}},
		{"hash without space", "s:\n  k: a#b", map[string]any{"s": map[string]any{"k": "a#b"}}},
		{"comment after value", "s:\n  k: v   # note", map[string]any{"s": map[string]any{"k": "v"}}},
		{"hash in quotes", "s:\n  k: \"a # b\"", map[string]any{"s": map[string]any{"k": "a # b"}}},
		{"equals", "s:\n  k: APP=1", map[string]any{"s": map[string]any{"k": "APP=1"}}},
		{"braces inside", "s:\n  k: v{version}", map[string]any{"s": map[string]any{"k": "v{version}"}}},
		{"quotes inside plain", "s:\n  k: ssh host 'cd /app && up'", map[string]any{"s": map[string]any{"k": "ssh host 'cd /app && up'"}}},
		{"booleans", "s:\n  a: true\n  b: false", map[string]any{"s": map[string]any{"a": true, "b": false}}},
		{"quoted boolean is text", "s:\n  a: \"true\"", map[string]any{"s": map[string]any{"a": "true"}}},
		{"quoted key", "s:\n  \"k\": v", map[string]any{"s": map[string]any{"k": "v"}}},
		{"four spaces", "s:\n    k: v", map[string]any{"s": map[string]any{"k": "v"}}},
		{"crlf", "s:\r\n  k: v\r\n", map[string]any{"s": map[string]any{"k": "v"}}},
		{"blank lines", "s:\n\n  a: x\n\n  b: y\n", map[string]any{"s": map[string]any{"a": "x", "b": "y"}}},
		{"flow list", "s:\n  k: [a, \"b, c\", 'd']", map[string]any{"s": map[string]any{"k": []any{"a", "b, c", "d"}}}},
		{"block list", "s:\n  k:\n    - a\n    - \"b\"\n", map[string]any{"s": map[string]any{"k": []any{"a", "b"}}}},
		{"block list at key indent", "s:\n  k:\n  - a\n  - b\n", map[string]any{"s": map[string]any{"k": []any{"a", "b"}}}},
		{"block list then key", "s:\n  k:\n    - a\n  next: v\n", map[string]any{"s": map[string]any{"k": []any{"a"}, "next": "v"}}},
		{"list item with comment", "s:\n  k:\n    - a # one\n    - b\n", map[string]any{"s": map[string]any{"k": []any{"a", "b"}}}},
		{
			"two sections",
			"a:\n  k: 1x\nb:\n  k: 2x\n",
			map[string]any{"a": map[string]any{"k": "1x"}, "b": map[string]any{"k": "2x"}},
		},
		{"top-level key", "k: v", map[string]any{"k": "v"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, parseError := parseYAML(testCase.content)
			if parseError != nil {
				t.Fatalf("parseYAML: %v", parseError)
			}

			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("got  %#v\nwant %#v", got, testCase.want)
			}
		})
	}
}

func TestYAMLErrors(t *testing.T) {
	cases := map[string]string{
		"tab indent":          "s:\n\tk: v",
		"no colon":            "s:\n  just words",
		"no space after":      "s:\n  k:v",
		"missing value":       "s:\n  k:",
		"missing value eof":   "s:",
		"bad indent":          "s:\n  a: x\n    b: y",
		"dedent mismatch":     "s:\n    a: x\n  b: y",
		"anchor":              "s:\n  k: &a v",
		"alias":               "s:\n  k: *a",
		"block scalar":        "s:\n  k: |",
		"folded scalar":       "s:\n  k: >",
		"flow map":            "s:\n  k: {a: b}",
		"tag":                 "s:\n  k: !!str v",
		"unclosed flow list":  "s:\n  k: [a, b",
		"unclosed quote":      "s:\n  k: \"a",
		"text after quote":    "s:\n  k: \"a\" b",
		"duplicate key":       "s:\n  k: a\n  k: b",
		"empty list item":     "s:\n  k:\n    - \n",
		"list at root":        "- a\n- b",
		"bad escape":          "s:\n  k: \"\\x41\"",
		"nested in list item": "s:\n  k:\n    - a: b\n      c: d",
		"map in list item":    "s:\n  k:\n    - a: b",
		"map in value":        "s:\n  k: a: b",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, parseError := parseYAML(content)
			if parseError == nil {
				t.Fatalf("want an error for %q", content)
			}
		})
	}
}

func TestYAMLErrorLine(t *testing.T) {
	_, parseError := parseYAML("s:\n  a: x\n\n  b: &bad\n")
	if parseError == nil || !strings.HasPrefix(parseError.Error(), "line 4:") {
		t.Fatalf("want an error on line 4, got %v", parseError)
	}
}

func TestYAMLListAtKeyIndent(t *testing.T) {
	config, parseError := Parse("publish:\n  command:\n  - npm publish\n  - echo done\n")
	if parseError != nil {
		t.Fatal(parseError)
	}

	if len(config.Publish.Command) != 2 {
		t.Fatalf("command = %q", config.Publish.Command)
	}
}

func TestYAMLBooleans(t *testing.T) {
	config, parseError := Parse("publish:\n  push: false\n")
	if parseError != nil {
		t.Fatal(parseError)
	}

	if config.Publish.Push.Enabled() {
		t.Fatal("push should be off")
	}
}
