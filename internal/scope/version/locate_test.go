package version

import "testing"

func TestLocate(t *testing.T) {
	cases := []struct {
		name    string
		format  Format
		content string
		path    string
		want    string
	}{
		{
			"json root", FORMAT_JSON,
			"{\n    \"name\": \"app\",\n    \"version\": \"1.2.3\",\n    \"dependencies\": {\"x\": \"9.9.9\"}\n}\n",
			"version", "1.2.3",
		},
		{
			"json skips nested", FORMAT_JSON,
			`{"dependencies": {"version": "0.0.1"}, "version": "1.2.3"}`,
			"version", "1.2.3",
		},
		{
			"json escapes", FORMAT_JSON,
			`{"description": "a \"quoted\" {x}", "version": "2.0.0"}`,
			"version", "2.0.0",
		},
		{
			"toml table", FORMAT_TOML,
			"[project]\nname = \"app\"\nversion = \"1.2.3\" # current\n\n[tool.other]\nversion = \"9\"\n",
			"project.version", "1.2.3",
		},
		{
			"toml nested table", FORMAT_TOML,
			"[workspace.package]\nversion = '0.4.0'\n",
			"workspace.package.version", "0.4.0",
		},
		{
			"toml dotted key", FORMAT_TOML,
			"package.version = \"3.1.4\"\n",
			"package.version", "3.1.4",
		},
		{
			"toml skips multiline", FORMAT_TOML,
			"[package]\ndescription = \"\"\"\nversion = \"no\"\n\"\"\"\nversion = \"1.0.0\"\n",
			"package.version", "1.0.0",
		},
		{
			"yaml root", FORMAT_YAML,
			"name: app\nversion: 1.2.3+4 # build\ndependencies:\n  version: 9.9.9\n",
			"version", "1.2.3+4",
		},
		{
			"yaml quoted", FORMAT_YAML,
			"version: \"1.2.3\"\n",
			"version", "1.2.3",
		},
		{
			"yaml nested", FORMAT_YAML,
			"app:\n  meta:\n    version: 0.1.0\n  other: x\n",
			"app.meta.version", "0.1.0",
		},
		{
			"xml pom", FORMAT_XML,
			"<?xml version=\"1.0\"?>\n<project>\n  <parent>\n    <version>9.9.9</version>\n  </parent>\n  <!-- <version>0</version> -->\n  <version>1.2.3</version>\n</project>\n",
			"project.version", "1.2.3",
		},
		{
			"xml csproj relative", FORMAT_XML,
			"<Project Sdk=\"Microsoft.NET.Sdk\">\n  <PropertyGroup>\n    <TargetFramework>net8.0</TargetFramework>\n    <Version>2.3.4</Version>\n  </PropertyGroup>\n</Project>\n",
			"PropertyGroup.Version", "2.3.4",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			span, isFound := Locate(testCase.format, testCase.content, testCase.path)
			if !isFound {
				t.Fatalf("%s not found", testCase.path)
			}

			got := testCase.content[span.Start:span.End]
			if got != testCase.want {
				t.Fatalf("got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestLocateMissing(t *testing.T) {
	cases := map[Format]string{
		FORMAT_JSON: `{"name": "x"}`,
		FORMAT_TOML: "[project]\ndynamic = [\"version\"]\n",
		FORMAT_YAML: "name: x\n",
		FORMAT_XML:  "<project><name>x</name></project>",
	}

	for format, content := range cases {
		_, isFound := Locate(format, content, "project.version")
		if isFound {
			t.Fatalf("%s: version should be missing", format)
		}
	}
}

func TestLocateEdges(t *testing.T) {
	found := []struct {
		name    string
		format  Format
		content string
		path    string
		want    string
	}{
		{"json after array", FORMAT_JSON, `{"files": ["a", {"version": "9"}], "keywords": [], "version": "1.0.0"}`, "version", "1.0.0"},
		{"json after numbers", FORMAT_JSON, `{"private": true, "count": 3, "nothing": null, "version": "1.0.0"}`, "version", "1.0.0"},
		{"json nested", FORMAT_JSON, `{"a": {"b": {"version": "4.0.0"}}}`, "a.b.version", "4.0.0"},
		{"xml self-closing and cdata", FORMAT_XML, "<project><empty/><description><![CDATA[<version>0</version>]]></description><version>1.0.0</version></project>", "project.version", "1.0.0"},
		{"xml doctype and spaces", FORMAT_XML, "<!DOCTYPE x>\n<project>\n  <version>\n    1.0.0\n  </version>\n</project>", "project.version", "1.0.0"},
		{"xml namespace", FORMAT_XML, `<p:project xmlns:p="x"><p:version>1.0.0</p:version></p:project>`, "project.version", "1.0.0"},
		{"toml comment on header", FORMAT_TOML, "[package] # main\nversion = \"1.0.0\"\n", "package.version", "1.0.0"},
		{"toml quoted key", FORMAT_TOML, "[package]\n\"version\" = \"1.0.0\"\n", "package.version", "1.0.0"},
		{"yaml list skipped", FORMAT_YAML, "authors:\n  - a\nversion: 1.0.0\n", "version", "1.0.0"},
	}

	for _, testCase := range found {
		t.Run(testCase.name, func(t *testing.T) {
			span, isFound := Locate(testCase.format, testCase.content, testCase.path)
			if !isFound || testCase.content[span.Start:span.End] != testCase.want {
				t.Fatalf("found %v: %q", isFound, testCase.content[span.Start:span.End])
			}
		})
	}

	missing := []struct {
		name    string
		format  Format
		content string
		path    string
	}{
		{"json number", FORMAT_JSON, `{"version": 1}`, "version"},
		{"json broken", FORMAT_JSON, `{"version": "1.0.0"`, "nope"},
		{"json bad key", FORMAT_JSON, `{version: "1"}`, "version"},
		{"json no colon", FORMAT_JSON, `{"version" "1"}`, "version"},
		{"json unclosed string", FORMAT_JSON, `{"version": "1`, "version"},
		{"json unclosed array", FORMAT_JSON, `{"a": [1, 2`, "version"},
		{"json empty", FORMAT_JSON, ``, "version"},
		{"toml number", FORMAT_TOML, "[package]\nversion = 1\n", "package.version"},
		{"toml unclosed", FORMAT_TOML, "[package]\nversion = \"1\n", "package.version"},
		{"yaml empty value", FORMAT_YAML, "version:\n", "version"},
		{"yaml unclosed quote", FORMAT_YAML, "version: \"1\n", "version"},
		{"xml empty element", FORMAT_XML, "<project><version></version></project>", "project.version"},
		{"xml unclosed comment", FORMAT_XML, "<project><!-- <version>1</version></project>", "project.version"},
		{"xml unclosed cdata", FORMAT_XML, "<project><![CDATA[x</project>", "project.version"},
		{"xml unclosed doctype", FORMAT_XML, "<!DOCTYPE", "project.version"},
		{"xml unclosed tag", FORMAT_XML, "<project><version", "project.version"},
	}

	for _, testCase := range missing {
		t.Run(testCase.name, func(t *testing.T) {
			_, isFound := Locate(testCase.format, testCase.content, testCase.path)
			if isFound {
				t.Fatal("must not be found")
			}
		})
	}
}
