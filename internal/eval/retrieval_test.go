package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// The three retrieval fixtures predate distractors and their numbers are already
// recorded in docs/data. If the builder's output moves, those results stop being
// comparable with anything measured afterwards — so the prompts are pinned, and a
// change here has to be a deliberate one that also re-runs the matrix.
func TestHaystackUnchangedForPreDistractorFixtures(t *testing.T) {
	for _, tc := range []struct {
		r    Retrieval
		want string
	}{
		{Retrieval{DepthTokens: 2000, Position: 0.5, Sentinel: "KEY-7F3A-2000"}, "0762ba25617909843a0b179d0d60e717"},
		{Retrieval{DepthTokens: 8000, Position: 0.5, Sentinel: "KEY-9B2C-8000"}, "8417b36cf02d2e2cf77f49f135365509"},
		{Retrieval{DepthTokens: 16000, Position: 0.5, Sentinel: "KEY-4D8E-16000"}, "c4bb038ba8ff0e2d2dd6db5329f9adc5"},
	} {
		sum := sha256.Sum256([]byte(buildHaystack(tc.r)))
		if got := hex.EncodeToString(sum[:])[:32]; got != tc.want {
			t.Errorf("depth %d: haystack hash %s, want %s — prompts moved under recorded results",
				tc.r.DepthTokens, got, tc.want)
		}
	}
}

func distractorTask() Retrieval {
	return Retrieval{
		DepthTokens: 2000, Position: 0.62, Sentinel: "KEY-STAGING-4417", Label: "staging",
		Distractors: []Decoy{
			{Label: "production", Sentinel: "KEY-PRODUCTION-4418", Position: 0.18},
			{Label: "development", Sentinel: "KEY-DEVELOPMENT-4419", Position: 0.88},
		},
	}
}

func TestHaystackPlantsEveryKeyInPositionOrder(t *testing.T) {
	h := buildHaystack(distractorTask())
	prod, stag, dev := strings.Index(h, "KEY-PRODUCTION-4418"), strings.Index(h, "KEY-STAGING-4417"), strings.Index(h, "KEY-DEVELOPMENT-4419")
	if prod < 0 || stag < 0 || dev < 0 {
		t.Fatalf("a planted key is missing: prod=%d staging=%d dev=%d", prod, stag, dev)
	}
	if prod >= stag || stag >= dev {
		t.Errorf("keys are not in position order: prod=%d staging=%d dev=%d", prod, stag, dev)
	}
	if !strings.Contains(h, "the staging deployment key is KEY-STAGING-4417") {
		t.Error("the wanted key is not labelled, so the question could not name it by property")
	}
}

func TestHaystackWithDistractorsIsDeterministic(t *testing.T) {
	// Two independently constructed but equal inputs, not one expression compared to
	// itself: the property is that equal tasks give equal prompts across separate runs,
	// which is what lets two configs be compared at all.
	a := buildHaystack(distractorTask())
	b := buildHaystack(distractorTask())
	if a != b {
		t.Error("two builds of one task differ; configs would be compared against different prompts")
	}
}

// The point of the shape: an answer is wrong if it names a decoy, even when the
// right key is also in there. Reciting the whole dump is a failure to discriminate,
// not a hedge that earns a pass.
func TestCheckRetrievalRejectsDistractors(t *testing.T) {
	r := distractorTask()
	for _, tc := range []struct {
		name, answer string
		want         Outcome
	}{
		{"wanted key alone", "KEY-STAGING-4417", Pass},
		{"decoy alone", "KEY-PRODUCTION-4418", FailRetrieval},
		{"every key listed", "staging KEY-STAGING-4417, prod KEY-PRODUCTION-4418, dev KEY-DEVELOPMENT-4419", FailRetrieval},
		{"nothing recalled", "I could not find it", FailRetrieval},
		{"empty", "   ", FailEmpty},
	} {
		if got, detail := checkRetrieval(r, tc.answer); got != tc.want {
			t.Errorf("%s: outcome %q, want %q (%s)", tc.name, got, tc.want, detail)
		}
	}
}

// Without distractors the check must behave exactly as it did before.
func TestCheckRetrievalUnchangedWithoutDistractors(t *testing.T) {
	r := Retrieval{DepthTokens: 2000, Position: 0.5, Sentinel: "KEY-7F3A-2000"}
	if got, _ := checkRetrieval(r, "KEY-7F3A-2000"); got != Pass {
		t.Errorf("plain recall no longer passes: %q", got)
	}
	if got, _ := checkRetrieval(r, "KEY-0000-0000"); got != FailRetrieval {
		t.Errorf("wrong key no longer fails: %q", got)
	}
}
