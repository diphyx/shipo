package semver

import "testing"

func TestCompare(t *testing.T) {
	order := []string{"0.9.9", "1.0.0-beta", "1.0.0", "1.0.1", "1.10.0", "2.0.0"}
	for index := 1; index < len(order); index++ {
		lower, _ := Parse(order[index-1])
		higher, _ := Parse(order[index])

		if lower.Compare(higher) != -1 || higher.Compare(lower) != 1 {
			t.Fatalf("%s should sort before %s", lower, higher)
		}
	}
}

func TestCompareEqual(t *testing.T) {
	left, _ := Parse("1.0.0+1")
	right, _ := Parse("1.0.0+2")

	if left.Compare(right) != 0 {
		t.Fatal("build metadata doesn't order versions")
	}
}

func TestComparePrerelease(t *testing.T) {
	ordered := []string{"1.0.0-1", "1.0.0-2", "1.0.0-10", "1.0.0-a", "1.0.0-a.1", "1.0.0-b", "1.0.0-next.2", "1.0.0-next.10", "1.0.0"}

	for index := 1; index < len(ordered); index++ {
		lower, _ := Parse(ordered[index-1])
		higher, _ := Parse(ordered[index])

		if lower.Compare(higher) != -1 || higher.Compare(lower) != 1 {
			t.Fatalf("want %s < %s", lower, higher)
		}
	}
}
