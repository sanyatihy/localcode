package eval

import (
	"context"
	"path/filepath"
	"testing"
)

// A discriminating task is a claim about two things: that a correct answer passes,
// and that the tempting wrong one does not. Both halves are checkable without a
// model, and checking them here is what stops a calibration run from spending an
// hour discovering that a fixture never had a trap in it — or worse, that its
// unseen test rejects a correct fix and scores the model down for being right.
//
// The wrong answers below are not strawmen. Each is the fix the reported symptom
// invites: it resolves exactly what was described and nothing that was not.
func TestPatchFixturesDiscriminate(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs go test per case")
	}
	for _, tc := range []struct {
		fixture string
		correct string
		// alsoCorrect are answers a competent engineer might legitimately write instead.
		// A fixture that fails one of these has more than one defensible action and is
		// scoring taste, which ranks nothing and costs a config marks for being right.
		alsoCorrect map[string]string
		wrong       map[string]string // label -> answer that must fail
	}{
		{
			fixture: "patch-off-by-one",
			correct: `package main

func Window(xs []int, n int) []int {
	if n <= 0 {
		return []int{}
	}
	if n > len(xs) {
		n = len(xs)
	}
	return xs[len(xs)-n:]
}`,
			alsoCorrect: map[string]string{
				// nil is an empty slice: len 0, ranges and appends the same, and the doc
				// comment draws no distinction. Scoring it would fail an answer that is
				// right about the off-by-one, which is the only thing this task asks.
				"nil for the empty cases": `package main

func Window(xs []int, n int) []int {
	if n <= 0 {
		return nil
	}
	if n > len(xs) {
		n = len(xs)
	}
	return xs[len(xs)-n:]
}`,
				"early return of the whole slice": `package main

func Window(xs []int, n int) []int {
	if n <= 0 {
		return []int{}
	}
	if n >= len(xs) {
		return xs
	}
	return xs[len(xs)-n:]
}`,
			},
			wrong: map[string]string{
				"over-corrects the other way": `package main

func Window(xs []int, n int) []int {
	if n <= 0 {
		return []int{}
	}
	if n > len(xs) {
		n = len(xs)
	}
	return xs[len(xs)-n-1:]
}`,
			},
		},
		{
			fixture: "patch-nil-check",
			correct: `package main

import "errors"

type Token struct {
	Value   string
	Expired bool
}

var ErrNilToken = errors.New("nil token")

func RefreshToken(tok *Token) (*Token, error) {
	if tok == nil {
		return nil, ErrNilToken
	}
	if tok.Expired {
		return &Token{Value: tok.Value + "-renewed", Expired: false}, nil
	}
	return tok, nil
}`,
			wrong: map[string]string{
				// Fair to fail: the prompt says to leave all other behaviour unchanged,
				// and the original returns the caller's pointer.
				"returns a copy of a valid token": `package main

import "errors"

type Token struct {
	Value   string
	Expired bool
}

var ErrNilToken = errors.New("nil token")

func RefreshToken(tok *Token) (*Token, error) {
	if tok == nil {
		return nil, ErrNilToken
	}
	if tok.Expired {
		return &Token{Value: tok.Value + "-renewed", Expired: false}, nil
	}
	return &Token{Value: tok.Value, Expired: tok.Expired}, nil
}`,
			},
		},
		{
			fixture: "patch-sibling-merge",
			correct: `package main

func Merge(defaults, overrides map[string]string) map[string]string {
	out := make(map[string]string, len(defaults)+len(overrides))
	for k, v := range defaults {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}`,
			wrong: map[string]string{
				"defaults the nil map and keeps aliasing": `package main

func Merge(defaults, overrides map[string]string) map[string]string {
	if defaults == nil {
		defaults = map[string]string{}
	}
	out := defaults
	for k, v := range overrides {
		out[k] = v
	}
	return out
}`,
				"guards nil by returning overrides": `package main

func Merge(defaults, overrides map[string]string) map[string]string {
	if defaults == nil {
		return overrides
	}
	for k, v := range overrides {
		defaults[k] = v
	}
	return defaults
}`,
			},
		},
		{
			fixture: "patch-sibling-splitpath",
			correct: `package main

import "strings"

func SplitPath(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}`,
			wrong: map[string]string{
				"trims the leading slash only": `package main

import "strings"

func SplitPath(p string) []string {
	return strings.Split(strings.TrimPrefix(p, "/"), "/")
}`,
				"trims both ends": `package main

import "strings"

func SplitPath(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}`,
			},
		},
		{
			fixture: "patch-contradiction-rounding",
			correct: `package main

import "math"

func RoundHalf(x float64) int {
	return int(math.Round(x))
}`,
			wrong: map[string]string{
				"satisfies both shown examples and neither rule": `package main

import "math"

func RoundHalf(x float64) int {
	if x == 2.5 {
		return 2
	}
	return int(math.Round(x))
}`,
				"truncates toward zero": `package main

func RoundHalf(x float64) int {
	return int(x)
}`,
			},
		},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			p := Patch{
				Dir:      filepath.Join("..", "..", "tasks", tc.fixture),
				Source:   "broken.go.txt",
				TestFile: "verify_test.go.txt",
			}
			if got, detail := runPatch(context.Background(), p, tc.correct); got != Pass {
				t.Errorf("a correct answer scored %q (%s) — the fixture punishes being right", got, detail)
			}
			for label, code := range tc.alsoCorrect {
				if got, detail := runPatch(context.Background(), p, code); got != Pass {
					t.Errorf("the equally-correct answer that uses %s scored %q (%s) — "+
						"this task has more than one defensible action and fails one", label, got, detail)
				}
			}
			for label, code := range tc.wrong {
				got, _ := runPatch(context.Background(), p, code)
				if got == Pass {
					t.Errorf("the wrong answer that %s passed — this task has no trap", label)
				}
			}
		})
	}
}

// The contradiction task accepts either resolution, which is a property worth
// asserting separately: a fixture that only accepted the author's preferred reading
// would be scoring taste rather than consistency.
func TestContradictionFixtureAcceptsEitherResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs go test per case")
	}
	p := Patch{
		Dir:      filepath.Join("..", "..", "tasks", "patch-contradiction-rounding"),
		Source:   "broken.go.txt",
		TestFile: "verify_test.go.txt",
	}
	for label, code := range map[string]string{
		"half away from zero, per the doc comment": `package main

import "math"

func RoundHalf(x float64) int {
	return int(math.Round(x))
}`,
		"half to even, per the existing test": `package main

import "math"

func RoundHalf(x float64) int {
	return int(math.RoundToEven(x))
}`,
	} {
		if got, detail := runPatch(context.Background(), p, code); got != Pass {
			t.Errorf("%s scored %q (%s); both readings must be accepted", label, got, detail)
		}
	}
}
