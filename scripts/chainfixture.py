"""Write the fixture a chain is measured on, buggy or fixed.

Generated rather than committed, for the reason a ladder rung is: two chains are only
comparable if they started from the same repository, and a directory copied by hand is a
claim about that rather than a guarantee. Both sides of a comparison materialise from here,
so they are identical by construction.

Twenty independent bugs, one per file, each with one test of its own. Independent because a
chain is being measured and not a model's reasoning: work that collapses into a single
insight would be finished in one session at any context, which is the one thing this cannot
answer. One file per bug because `Edit` refuses a file the session has not read, so the
files are what sets how much a session can get through before its ceiling.

    python3 scripts/chainfixture.py <dir>            the fixture, as the model is shown it
    python3 scripts/chainfixture.py <dir> --fixed    the same with every bug repaired

The second is the discrimination check, and it is why this file holds both: a fixture whose
tests do not fail before and pass after is measuring nothing, and finding that out from a
chain costs an hour.
"""

import os
import sys

MODULE = "fixme"

# name: (buggy body, fixed body, test body). The test never names which line is wrong — a
# session has to read the file — but it does pin the behaviour exactly, because the score
# has to be a fact rather than a judgement.
#
# Imports are derived from the body rather than listed, because they differ between the two
# variants: the buggy Join concatenates and the fixed one calls strings.Join, and a package
# imported and not used is a compile error rather than a failing test.
BUGS = {
    "take": ('''
// Take returns the first n elements of xs.
func Take(xs []int, n int) []int {
	if n > len(xs) {
		n = len(xs)
	}
	return xs[:n+1]
}
''', '''
// Take returns the first n elements of xs.
func Take(xs []int, n int) []int {
	if n > len(xs) {
		n = len(xs)
	}
	return xs[:n]
}
''', '''
	if got := Take([]int{1, 2, 3}, 2); !equal(got, []int{1, 2}) {
		t.Fatalf("Take(1,2,3 / 2) = %v, want [1 2]", got)
	}
'''),
    "count": ('''
// Count tallies how often each word appears.
func Count(words []string) map[string]int {
	var m map[string]int
	for _, w := range words {
		m[w]++
	}
	return m
}
''', '''
// Count tallies how often each word appears.
func Count(words []string) map[string]int {
	m := map[string]int{}
	for _, w := range words {
		m[w]++
	}
	return m
}
''', '''
	if got := Count([]string{"a", "a", "b"}); got["a"] != 2 || got["b"] != 1 {
		t.Fatalf("Count(a,a,b) = %v, want a:2 b:1", got)
	}
'''),
    "length": ('''
// Length is the length of the string s points at, and 0 for no string at all.
func Length(s *string) int {
	return len(*s)
}
''', '''
// Length is the length of the string s points at, and 0 for no string at all.
func Length(s *string) int {
	if s == nil {
		return 0
	}
	return len(*s)
}
''', '''
	if got := Length(nil); got != 0 {
		t.Fatalf("Length(nil) = %d, want 0", got)
	}
	s := "abc"
	if got := Length(&s); got != 3 {
		t.Fatalf("Length(abc) = %d, want 3", got)
	}
'''),
    "atmost": ('''
// AtMost keeps the elements of xs that do not exceed limit.
func AtMost(xs []int, limit int) []int {
	out := []int{}
	for _, x := range xs {
		if x < limit {
			out = append(out, x)
		}
	}
	return out
}
''', '''
// AtMost keeps the elements of xs that do not exceed limit.
func AtMost(xs []int, limit int) []int {
	out := []int{}
	for _, x := range xs {
		if x <= limit {
			out = append(out, x)
		}
	}
	return out
}
''', '''
	if got := AtMost([]int{1, 2, 3}, 2); !equal(got, []int{1, 2}) {
		t.Fatalf("AtMost(1,2,3 / 2) = %v, want [1 2]", got)
	}
'''),
    "with": ('''
// With returns xs plus x, leaving xs and every other result of With untouched.
func With(xs []int, x int) []int {
	return append(xs, x)
}
''', '''
// With returns xs plus x, leaving xs and every other result of With untouched.
func With(xs []int, x int) []int {
	out := make([]int, len(xs), len(xs)+1)
	copy(out, xs)
	return append(out, x)
}
''', '''
	base := make([]int, 1, 8)
	base[0] = 1
	first := With(base, 2)
	With(base, 3)
	if first[1] != 2 {
		t.Fatalf("the second With overwrote the first: %v", first)
	}
'''),
    "average": ('''
// Average is the mean of xs, and 0 for no elements.
func Average(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return float64(sum / len(xs))
}
''', '''
// Average is the mean of xs, and 0 for no elements.
func Average(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return float64(sum) / float64(len(xs))
}
''', '''
	if got := Average([]int{1, 2}); got != 1.5 {
		t.Fatalf("Average(1,2) = %v, want 1.5", got)
	}
'''),
    "minmax": ('''
// MinMax returns the smallest and largest of xs, in that order.
func MinMax(xs []int) (int, int) {
	lo, hi := xs[0], xs[0]
	for _, x := range xs {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	return hi, lo
}
''', '''
// MinMax returns the smallest and largest of xs, in that order.
func MinMax(xs []int) (int, int) {
	lo, hi := xs[0], xs[0]
	for _, x := range xs {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	return lo, hi
}
''', '''
	lo, hi := MinMax([]int{3, 1, 2})
	if lo != 1 || hi != 3 {
		t.Fatalf("MinMax(3,1,2) = %d, %d, want 1, 3", lo, hi)
	}
'''),
    "parseall": ('''
// ParseAll converts every element of ss, and reports the first one that is not a number.
func ParseAll(ss []string) ([]int, error) {
	out := []int{}
	for _, s := range ss {
		n, _ := strconv.Atoi(s)
		out = append(out, n)
	}
	return out, nil
}
''', '''
// ParseAll converts every element of ss, and reports the first one that is not a number.
func ParseAll(ss []string) ([]int, error) {
	out := []int{}
	for _, s := range ss {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}
''', '''
	if _, err := ParseAll([]string{"1", "x"}); err == nil {
		t.Fatal("ParseAll(1,x) returned no error")
	}
	got, err := ParseAll([]string{"1", "2"})
	if err != nil || !equal(got, []int{1, 2}) {
		t.Fatalf("ParseAll(1,2) = %v, %v, want [1 2], nil", got, err)
	}
'''),
    "first": ('''
// First is the first character of s, and "" for an empty string.
func First(s string) string {
	if s == "" {
		return ""
	}
	return string(s[0])
}
''', '''
// First is the first character of s, and "" for an empty string.
func First(s string) string {
	if s == "" {
		return ""
	}
	return string([]rune(s)[0])
}
''', '''
	if got := First("\\u65e5\\u672c"); got != "\\u65e5" {
		t.Fatalf("First = %q, want the first character", got)
	}
'''),
    "isempty": ('''
// IsEmpty reports whether s has no characters in it.
func IsEmpty(s string) bool {
	return len(s) != 0
}
''', '''
// IsEmpty reports whether s has no characters in it.
func IsEmpty(s string) bool {
	return len(s) == 0
}
''', '''
	if !IsEmpty("") || IsEmpty("a") {
		t.Fatalf("IsEmpty: got %v for empty and %v for non-empty", IsEmpty(""), IsEmpty("a"))
	}
'''),
    "middle": ('''
// Middle drops the first and last elements of xs.
func Middle(xs []int) []int {
	if len(xs) < 2 {
		return []int{}
	}
	return xs[1:len(xs)]
}
''', '''
// Middle drops the first and last elements of xs.
func Middle(xs []int) []int {
	if len(xs) < 2 {
		return []int{}
	}
	return xs[1 : len(xs)-1]
}
''', '''
	if got := Middle([]int{1, 2, 3, 4}); !equal(got, []int{2, 3}) {
		t.Fatalf("Middle(1,2,3,4) = %v, want [2 3]", got)
	}
'''),
    "product": ('''
// Product multiplies every element of xs, and is 1 for no elements.
func Product(xs []int) int {
	p := 0
	for _, x := range xs {
		p *= x
	}
	return p
}
''', '''
// Product multiplies every element of xs, and is 1 for no elements.
func Product(xs []int) int {
	p := 1
	for _, x := range xs {
		p *= x
	}
	return p
}
''', '''
	if got := Product([]int{2, 3}); got != 6 {
		t.Fatalf("Product(2,3) = %d, want 6", got)
	}
	if got := Product(nil); got != 1 {
		t.Fatalf("Product() = %d, want 1", got)
	}
'''),
    "contains": ('''
// Contains reports whether v is anywhere in xs.
func Contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
		return false
	}
	return false
}
''', '''
// Contains reports whether v is anywhere in xs.
func Contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
''', '''
	if !Contains([]int{1, 2, 3}, 3) {
		t.Fatal("Contains(1,2,3 / 3) = false, want true")
	}
	if Contains([]int{1, 2}, 9) {
		t.Fatal("Contains(1,2 / 9) = true, want false")
	}
'''),
    "getor": ('''
// GetOr is m[k] when the key is present, and fallback when it is not. A key present with
// the value 0 is present.
func GetOr(m map[string]int, k string, fallback int) int {
	v := m[k]
	if v == 0 {
		return fallback
	}
	return v
}
''', '''
// GetOr is m[k] when the key is present, and fallback when it is not. A key present with
// the value 0 is present.
func GetOr(m map[string]int, k string, fallback int) int {
	if v, ok := m[k]; ok {
		return v
	}
	return fallback
}
''', '''
	if got := GetOr(map[string]int{"a": 0}, "a", 7); got != 0 {
		t.Fatalf("GetOr on a key present with value 0 = %d, want 0", got)
	}
	if got := GetOr(map[string]int{}, "b", 7); got != 7 {
		t.Fatalf("GetOr on an absent key = %d, want 7", got)
	}
'''),
    "trim": ('''
// Trim removes whitespace from both ends of s.
func Trim(s string) string {
	return strings.TrimLeft(s, " ")
}
''', '''
// Trim removes whitespace from both ends of s.
func Trim(s string) string {
	return strings.TrimSpace(s)
}
''', '''
	if got := Trim("  a  "); got != "a" {
		t.Fatalf("Trim = %q, want \\"a\\"", got)
	}
'''),
    "evens": ('''
// Evens keeps every even element of xs, in order.
func Evens(xs []int) []int {
	out := []int{}
	for _, x := range xs {
		if x%2 != 0 {
			break
		}
		out = append(out, x)
	}
	return out
}
''', '''
// Evens keeps every even element of xs, in order.
func Evens(xs []int) []int {
	out := []int{}
	for _, x := range xs {
		if x%2 != 0 {
			continue
		}
		out = append(out, x)
	}
	return out
}
''', '''
	if got := Evens([]int{2, 1, 4}); !equal(got, []int{2, 4}) {
		t.Fatalf("Evens(2,1,4) = %v, want [2 4]", got)
	}
'''),
    "mean3": ('''
// Mean3 is the average of its three arguments, rounded towards zero.
func Mean3(a, b, c int) int {
	return a + b + c/3
}
''', '''
// Mean3 is the average of its three arguments, rounded towards zero.
func Mean3(a, b, c int) int {
	return (a + b + c) / 3
}
''', '''
	if got := Mean3(3, 3, 3); got != 3 {
		t.Fatalf("Mean3(3,3,3) = %d, want 3", got)
	}
'''),
    "clamp": ('''
// Clamp holds v inside [lo, hi].
func Clamp(v, lo, hi int) int {
	if v < lo {
		return hi
	}
	if v > hi {
		return lo
	}
	return v
}
''', '''
// Clamp holds v inside [lo, hi].
func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
''', '''
	if got := Clamp(-1, 0, 10); got != 0 {
		t.Fatalf("Clamp(-1) = %d, want 0", got)
	}
	if got := Clamp(11, 0, 10); got != 10 {
		t.Fatalf("Clamp(11) = %d, want 10", got)
	}
'''),
    "wrap": ('''
// Wrap is i reduced into [0, n), for negative i as well as positive.
func Wrap(i, n int) int {
	return i % n
}
''', '''
// Wrap is i reduced into [0, n), for negative i as well as positive.
func Wrap(i, n int) int {
	return ((i % n) + n) % n
}
''', '''
	if got := Wrap(-1, 3); got != 2 {
		t.Fatalf("Wrap(-1, 3) = %d, want 2", got)
	}
	if got := Wrap(4, 3); got != 1 {
		t.Fatalf("Wrap(4, 3) = %d, want 1", got)
	}
'''),
    "join": ('''
// Join puts a comma between the elements of xs, and none after the last.
func Join(xs []string) string {
	out := ""
	for _, x := range xs {
		out += x + ","
	}
	return out
}
''', '''
// Join puts a comma between the elements of xs, and none after the last.
func Join(xs []string) string {
	return strings.Join(xs, ",")
}
''', '''
	if got := Join([]string{"a", "b"}); got != "a,b" {
		t.Fatalf("Join(a,b) = %q, want \\"a,b\\"", got)
	}
'''),
}

HEADER = "package " + MODULE + "\n"

TEST_PROLOGUE = '''package ''' + MODULE + '''

import "testing"

// equal compares two int slices, since the tests below do it repeatedly and a helper the
// session may not edit is one less thing for it to get wrong.
func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// survive turns a panicking bug into a failing test. Three of these bugs panic, and without
// this the first one ends the test binary and the other nineteen results are never printed —
// so the score would say which bug panicked rather than how many are fixed.
func survive(t *testing.T) {
	if r := recover(); r != nil {
		t.Fatalf("panicked: %v", r)
	}
}
'''


def write(dir_, fixed):
    os.makedirs(dir_, exist_ok=True)
    with open(os.path.join(dir_, "go.mod"), "w") as fh:
        fh.write("module " + MODULE + "\n\ngo 1.22\n")

    test = [TEST_PROLOGUE]
    for name in sorted(BUGS):
        buggy, repaired, body = BUGS[name]
        chosen = repaired if fixed else buggy
        src = HEADER
        for pkg in ("strconv", "strings"):
            if pkg + "." in chosen:
                src += '\nimport "' + pkg + '"\n'
        src += chosen
        with open(os.path.join(dir_, name + ".go"), "w") as fh:
            fh.write(src)
        test.append("\nfunc Test" + name.capitalize() + "(t *testing.T) {\n\tdefer survive(t)\n" + body + "}\n")

    with open(os.path.join(dir_, "fixme_test.go"), "w") as fh:
        fh.write("".join(test))
    return len(BUGS)


if __name__ == "__main__":
    if len(sys.argv) < 2:
        sys.exit("usage: chainfixture.py <dir> [--fixed]")
    n = write(sys.argv[1], "--fixed" in sys.argv[2:])
    print("%d bugs in %s%s" % (n, sys.argv[1], " (repaired)" if "--fixed" in sys.argv[2:] else ""))
