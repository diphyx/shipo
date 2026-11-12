package process

import (
	"errors"
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	cases := []struct {
		name, line string
		want       []string
	}{
		{"simple", "npm ci", []string{"npm", "ci"}},
		{"spaces", "  go   test  ./...  ", []string{"go", "test", "./..."}},
		{"double quotes", `go build -ldflags "-s -w"`, []string{"go", "build", "-ldflags", "-s -w"}},
		{"single quotes", `ssh host 'cd /app && up'`, []string{"ssh", "host", "cd /app && up"}},
		{"double inside single", `echo 'say "hi"'`, []string{"echo", `say "hi"`}},
		{"single inside double", `echo "it's"`, []string{"echo", "it's"}},
		{"escaped quote", `echo "a \"b\""`, []string{"echo", `a "b"`}},
		{"backslash kept in double", `echo "C:\path"`, []string{"echo", `C:\path`}},
		{"escape outside", `echo a\ b`, []string{"echo", "a b"}},
		{"empty quoted", `run ""`, []string{"run", ""}},
		{"joined", `title="a b"`, []string{"title=a b"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, splitError := Split(testCase.line)
			if splitError != nil {
				t.Fatalf("Split(%q) error: %v", testCase.line, splitError)
			}

			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("Split(%q) = %q, want %q", testCase.line, got, testCase.want)
			}
		})
	}
}

func TestSplitErrors(t *testing.T) {
	_, emptyError := Split("   ")
	if !errors.Is(emptyError, ErrEmpty) {
		t.Fatalf("want ErrEmpty, got %v", emptyError)
	}

	_, quoteError := Split(`echo "open`)
	if quoteError == nil {
		t.Fatal("want an unclosed quote error")
	}
}
