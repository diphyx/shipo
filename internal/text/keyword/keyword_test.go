package keyword

import "testing"

func TestExpand(t *testing.T) {
	values := Values{"version": "1.5.0", "tag": "v1.5.0"}

	cases := []struct {
		name, text, want string
	}{
		{"plain", "release", "release"},
		{"one", "v{version}", "v1.5.0"},
		{"two", "{tag} ({version})", "v1.5.0 (1.5.0)"},
		{"unknown kept", "{nope}-{version}", "{nope}-1.5.0"},
		{"escape", "{{version}", "{version}"},
		{"escape then keyword", "{{{version}", "{1.5.0"},
		{"unclosed", "a {version", "a {version"},
		{"empty name", "{}", "{}"},
		{"json-like", `{{"a": "{version}"}`, `{"a": "1.5.0"}`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Expand(testCase.text, values)
			if got != testCase.want {
				t.Fatalf("Expand(%q) = %q, want %q", testCase.text, got, testCase.want)
			}
		})
	}
}

func TestExpandAll(t *testing.T) {
	got := ExpandAll([]string{"APP={version}", "plain", "{nope}"}, Values{"version": "2.0.0"})

	if len(got) != 3 || got[0] != "APP=2.0.0" || got[1] != "plain" || got[2] != "{nope}" {
		t.Fatalf("got %q", got)
	}

	if len(ExpandAll(nil, Values{})) != 0 {
		t.Fatal("nil in, empty out")
	}
}
