package version

import "testing"

func TestTextPattern(t *testing.T) {
	pattern, compileError := ParsePattern(`VERSION{_}={_}"{version}"`)
	if compileError != nil {
		t.Fatal(compileError)
	}

	content := "#!/bin/sh\nVERSION = \"1.2.3\"\necho done\n"

	matches := pattern.FindAll(content)
	if len(matches) != 1 || matches[0].Version != "1.2.3" {
		t.Fatalf("matches = %+v", matches)
	}

	written := pattern.Replace(content, matches, Fill{Version: "1.3.0"})
	if written != "#!/bin/sh\nVERSION = \"1.3.0\"\necho done\n" {
		t.Fatalf("written = %q", written)
	}
}

func TestPatternFill(t *testing.T) {
	pattern, compileError := ParsePattern(`APP="{version} ({hash})" build={counter}`)
	if compileError != nil {
		t.Fatal(compileError)
	}

	content := `APP="1.0.0 (abc1234)" build=41`
	matches := pattern.FindAll(content)

	written := pattern.Replace(content, matches, Fill{Version: "1.0.1", Hash: "def5678"})
	if written != `APP="1.0.1 (def5678)" build=42` {
		t.Fatalf("written = %q", written)
	}
}

func TestPatternAnchors(t *testing.T) {
	pattern, compileError := ParsePattern(`^version = {version}$`)
	if compileError != nil {
		t.Fatal(compileError)
	}

	matches := pattern.FindAll("x version = 9.9.9\nversion = 1.2.3\n")
	if len(matches) != 1 || matches[0].Version != "1.2.3" {
		t.Fatalf("matches = %+v", matches)
	}
}

func TestPatternLiteral(t *testing.T) {
	pattern, compileError := ParsePattern(`v{version}.(stable)`)
	if compileError != nil {
		t.Fatal(compileError)
	}

	if len(pattern.FindAll("v1.2.3.(stable)")) != 1 || len(pattern.FindAll("v1.2.3x(stable)")) != 0 {
		t.Fatal("text mode must treat . and ( literally")
	}
}

func TestRegexPattern(t *testing.T) {
	pattern, compileError := ParsePattern(`/^const Version = "{version}"$/`)
	if compileError != nil {
		t.Fatal(compileError)
	}

	matches := pattern.FindAll("package main\n\nconst Version = \"0.9.0\"\n")
	if len(matches) != 1 || matches[0].Version != "0.9.0" {
		t.Fatalf("matches = %+v", matches)
	}
}

func TestPatternErrors(t *testing.T) {
	for _, source := range []string{"no keyword", "{version} {version}", "{nope} {version}", "{version"} {
		_, compileError := ParsePattern(source)
		if compileError == nil {
			t.Fatalf("ParsePattern(%q) should fail", source)
		}
	}
}

func TestTagPattern(t *testing.T) {
	pattern, compileError := ParseTag("api/v{version}")
	if compileError != nil {
		t.Fatal(compileError)
	}

	match, isMatch := pattern.MatchFull("api/v1.4.0")
	if !isMatch || match.Version != "1.4.0" {
		t.Fatalf("match = %+v %v", match, isMatch)
	}

	_, isOther := pattern.MatchFull("web/api/v1.4.0")
	if isOther {
		t.Fatal("tag pattern must match the whole name")
	}

	if Render("api/v{version}", Fill{Version: "1.5.0"}) != "api/v1.5.0" {
		t.Fatal("Render")
	}
}
