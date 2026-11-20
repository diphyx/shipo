package semver

import "testing"

func TestBump(t *testing.T) {
	version, parseError := Parse("1.2.3")
	if parseError != nil {
		t.Fatal(parseError)
	}

	want := map[Level]string{
		LEVEL_MAJOR:   "2.0.0",
		LEVEL_MINOR:   "1.3.0",
		LEVEL_PATCH:   "1.2.4",
		LEVEL_CURRENT: "1.2.3",
	}

	for level, expected := range want {
		next, bumpError := version.Bump(level)
		if bumpError != nil {
			t.Fatal(bumpError)
		}

		if next.String() != expected {
			t.Fatalf("%s: got %s, want %s", level, next, expected)
		}
	}
}

func TestParse(t *testing.T) {
	valid := []string{"0.0.0", "1.2.3", "10.20.30", "1.0.0-beta.1", "1.0.0+5", "1.0.0-rc.1+build.7"}
	for _, text := range valid {
		version, parseError := Parse(text)
		if parseError != nil {
			t.Fatalf("Parse(%q): %v", text, parseError)
		}

		if version.String() != text {
			t.Fatalf("String() = %q, want %q", version.String(), text)
		}
	}

	invalid := []string{"", "1.2", "v1.2.3", "1.2.3.4", "a.b.c"}
	for _, text := range invalid {
		_, parseError := Parse(text)
		if parseError == nil {
			t.Fatalf("Parse(%q) should fail", text)
		}
	}
}

func TestParseLevel(t *testing.T) {
	for _, name := range []string{"ask", "major", "minor", "patch", "current"} {
		level, parseError := ParseLevel(name)
		if parseError != nil || string(level) != name {
			t.Fatalf("ParseLevel(%q) = %q, %v", name, level, parseError)
		}
	}

	for _, name := range []string{"", "Patch", "rebuild", "huge"} {
		_, parseError := ParseLevel(name)
		if parseError == nil {
			t.Fatalf("ParseLevel(%q) should fail", name)
		}
	}
}

func TestBumpAskAndSuffix(t *testing.T) {
	version, _ := Parse("1.2.3-beta.1+7")

	_, askError := version.Bump(LEVEL_ASK)
	if askError == nil {
		t.Fatal("ask is not a bump")
	}

	next, _ := version.Bump(LEVEL_PATCH)
	current, _ := version.Bump(LEVEL_CURRENT)

	if next.String() != "1.2.4" || current.String() != "1.2.3-beta.1+7" {
		t.Fatalf("patch %s, current %s", next, current)
	}
}

func TestBumpFinal(t *testing.T) {
	version, _ := Parse("1.2.0-next+5")

	final, finalError := version.Bump(LEVEL_FINAL)
	if finalError != nil || final.String() != "1.2.0" {
		t.Fatalf("final = %s %v", final, finalError)
	}

	if _, parseError := ParseLevel("final"); parseError != nil {
		t.Fatal(parseError)
	}
}

func TestLevelsOrder(t *testing.T) {
	want := []Level{LEVEL_FINAL, LEVEL_PATCH, LEVEL_MINOR, LEVEL_MAJOR, LEVEL_CURRENT}
	for index, level := range want {
		if Levels[index] != level {
			t.Fatalf("menu order: %v", Levels)
		}
	}
}
