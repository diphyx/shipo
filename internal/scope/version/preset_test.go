package version

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNamesAndLookup(t *testing.T) {
	names := Names()
	if names[0] != PRESET_AUTO || names[len(names)-1] != PRESET_CUSTOM || len(names) != len(Presets)+2 {
		t.Fatalf("names = %v", names)
	}

	for _, preset := range Presets {
		found, isFound := Lookup(preset.Name)
		if !isFound || found.Name != preset.Name {
			t.Fatalf("Lookup(%s)", preset.Name)
		}
	}

	_, isFound := Lookup("cobol")
	if isFound {
		t.Fatal("unknown preset")
	}
}

func TestEveryPresetIsDetected(t *testing.T) {
	files := map[string]string{
		PRESET_JAVASCRIPT: "package.json",
		PRESET_PYTHON:     "pyproject.toml",
		PRESET_RUST:       "Cargo.toml",
		PRESET_PHP:        "composer.json",
		PRESET_DART:       "pubspec.yaml",
		PRESET_JAVA:       "pom.xml",
		PRESET_CSHARP:     "App.csproj",
		PRESET_GO:         "go.mod",
		PRESET_SWIFT:      "Package.swift",
	}

	for name, file := range files {
		dir := t.TempDir()
		writeFile(t, dir, file, "")

		preset, detectError := Detect(dir)
		if detectError != nil || preset.Name != name {
			t.Fatalf("%s: detected %q, %v", file, preset.Name, detectError)
		}
	}
}

func TestDetectFile(t *testing.T) {
	cases := map[string]string{
		"package.json":            PRESET_JAVASCRIPT,
		"packages/a/package.json": PRESET_JAVASCRIPT,
		"api/Cargo.toml":          PRESET_RUST,
		"src/App.csproj":          PRESET_CSHARP,
	}

	for source, want := range cases {
		preset, isFound := DetectFile(source)
		if !isFound || preset.Name != want {
			t.Fatalf("DetectFile(%s) = %q", source, preset.Name)
		}
	}

	_, isFound := DetectFile("VERSION")
	if isFound {
		t.Fatal("VERSION has no preset")
	}
}

func TestEveryPresetReadsItsFile(t *testing.T) {
	cases := map[string][2]string{
		PRESET_JAVASCRIPT: {"package.json", `{"version": "1.0.0"}`},
		PRESET_PYTHON:     {"pyproject.toml", "[project]\nversion = \"1.0.0\"\n"},
		PRESET_RUST:       {"Cargo.toml", "[package]\nname = \"x\"\nversion = \"1.0.0\"\n"},
		PRESET_PHP:        {"composer.json", `{"name": "x/y", "version": "1.0.0"}`},
		PRESET_DART:       {"pubspec.yaml", "name: x\nversion: 1.0.0\n"},
		PRESET_JAVA:       {"pom.xml", "<project><modelVersion>4.0.0</modelVersion><version>1.0.0</version></project>"},
		PRESET_CSHARP:     {"App.csproj", "<Project><PropertyGroup><Version>1.0.0</Version></PropertyGroup></Project>"},
	}

	for name, file := range cases {
		dir := t.TempDir()
		path := writeFile(t, dir, file[0], file[1])

		resolved, resolveError := Resolve(Settings{Preset: name}, dir)
		if resolveError != nil {
			t.Fatalf("%s: %v", name, resolveError)
		}

		current, readError := Read(resolved, nil)
		if readError != nil || current.Version.String() != "1.0.0" {
			t.Fatalf("%s: read %v, %v", name, current, readError)
		}

		_, writeError := current.Write(Fill{Version: "2.0.0"})
		if writeError != nil {
			t.Fatal(writeError)
		}

		if got := readBack(t, path); got != strings.Replace(file[1], "1.0.0", "2.0.0", 1) || strings.Contains(file[1], "4.0.0") && !strings.Contains(got, "4.0.0") {
			t.Fatalf("%s wrote:\n%s", name, got)
		}

		if len(current.Files()) != 1 || current.Resolved() != resolved {
			t.Fatal("Files and Resolved")
		}
	}
}

func TestResolveErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"version": "1.0.0"}`)
	_ = os.MkdirAll(filepath.Join(dir, "folder"), 0o755)

	cases := map[string]Settings{
		"unknown preset":         {Preset: "cobol"},
		"custom without pattern": {Preset: PRESET_CUSTOM, Source: "VERSION"},
		"custom without source":  {Preset: PRESET_CUSTOM, Pattern: "{version}"},
		"git with source":        {Preset: PRESET_GO, Source: "go.mod"},
		"pattern only":           {Pattern: "v{version}"},
		"unknown file":           {Source: "VERSION"},
		"no match":               {Source: "missing/package.json"},
		"only folders":           {Preset: PRESET_CUSTOM, Source: "folder", Pattern: "{version}"},
		"bad glob":               {Preset: PRESET_CUSTOM, Source: "[", Pattern: "{version}"},
		"bad tag pattern":        {Preset: PRESET_GIT, Pattern: "no keyword"},
	}

	for name, settings := range cases {
		t.Run(name, func(t *testing.T) {
			_, resolveError := Resolve(settings, dir)
			if resolveError == nil {
				t.Fatalf("want an error for %+v", settings)
			}
		})
	}
}

func TestAutoWithSourceAndPattern(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "VERSION", "3.1.4\n")

	resolved, resolveError := Resolve(Settings{Source: "VERSION", Pattern: "{version}"}, dir)
	if resolveError != nil || resolved.Preset != PRESET_CUSTOM {
		t.Fatalf("resolved = %+v, %v", resolved, resolveError)
	}

	current, _ := Read(resolved, nil)
	if current.Version.String() != "3.1.4" {
		t.Fatalf("version = %s", current.Version)
	}
}

func TestReadErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "VERSION", "not a version\n")
	writeFile(t, dir, "bad.sh", "V=\"1.0\"\n")

	resolved, _ := Resolve(Settings{Preset: PRESET_CUSTOM, Source: "VERSION", Pattern: "{version}"}, dir)

	_, noMatch := Read(resolved, nil)
	if noMatch == nil || !strings.Contains(noMatch.Error(), "no version found") {
		t.Fatalf("want no version found, got %v", noMatch)
	}

	badPattern := &Resolved{Preset: PRESET_CUSTOM, Files: []string{filepath.Join(dir, "bad.sh")}, Patterns: []string{"{nope}"}, Dir: dir}

	_, patternError := Read(badPattern, nil)
	if patternError == nil {
		t.Fatal("want a pattern error")
	}

	missing := &Resolved{Preset: PRESET_CUSTOM, Files: []string{filepath.Join(dir, "gone")}, Patterns: []string{"{version}"}, Dir: dir}

	_, missingError := Read(missing, nil)
	if missingError == nil {
		t.Fatal("want a read error")
	}
}

func TestWriteIsAllOrNone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions")
	}

	dir := t.TempDir()
	first := writeFile(t, dir, "packages/a/package.json", `{"version": "1.0.0"}`)
	second := writeFile(t, dir, "packages/b/package.json", `{"version": "1.0.0"}`)

	resolved, _ := Resolve(Settings{Source: "packages/*/package.json"}, dir)
	current, _ := Read(resolved, nil)

	_ = os.Chmod(second, 0o444)
	defer func() { _ = os.Chmod(second, 0o644) }()

	_, writeError := current.Write(Fill{Version: "2.0.0"})
	if writeError == nil {
		t.Skip("running as root: read-only files are still writable")
	}

	if readBack(t, first) != `{"version": "1.0.0"}` {
		t.Fatal("the first file must be restored when the second fails")
	}
}

func TestWriteSkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"version": "1.0.0"}`)

	resolved, _ := Resolve(Settings{}, dir)
	current, _ := Read(resolved, nil)

	files, writeError := current.Write(Fill{Version: "1.0.0"})
	if writeError != nil || len(files) != 0 {
		t.Fatalf("an unchanged version writes nothing: %v %v", files, writeError)
	}
}

func TestFormatOf(t *testing.T) {
	cases := map[string]Format{
		"package.json":          FORMAT_JSON,
		"pyproject.toml":        FORMAT_TOML,
		"pubspec.yaml":          FORMAT_YAML,
		"config.YML":            FORMAT_YAML,
		"pom.xml":               FORMAT_XML,
		"App.csproj":            FORMAT_XML,
		"App.fsproj":            FORMAT_XML,
		"Directory.Build.props": FORMAT_XML,
		"VERSION":               "",
		"version.sh":            "",
	}

	for file, want := range cases {
		if got := FormatOf(file); got != want {
			t.Fatalf("FormatOf(%s) = %q, want %q", file, got, want)
		}
	}

	_, isFound := Locate("ini", "version=1", "version")
	if isFound {
		t.Fatal("an unknown format locates nothing")
	}
}
