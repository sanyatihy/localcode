package prefix

import (
	"fmt"
	"strings"
	"testing"
)

// A request as the server logs one, kept verbatim from a run so the parser is tested
// against the shape it actually meets rather than against a tidied version of it.
const oneRequest = `
0.42.617.551 I slot get_availabl: id  0 | task -1 | selected slot by LCP similarity, f_sim_best = 0.702 (> 0.100 thold), f_keep = 0.998
0.42.617.733 I slot launch_slot_: id  0 | task 8 | processing task, is_child = 0
0.45.919.641 I slot print_timing: id  0 | task 8 | prompt processing, n_tokens =    822, progress = 1.00, t =   3.30 s / 248.95 tokens per second
0.51.025.995 I slot print_timing: id  0 | task 8 | prompt eval time =    8149.65 ms /   826 tokens (    9.87 ms per token,   101.35 tokens per second)
0.51.026.007 I slot print_timing: id  0 | task 8 |        eval time =     258.60 ms /     4 tokens (   86.20 ms per token,    11.60 tokens per second)
0.51.026.011 I slot print_timing: id  0 | task 8 |       total time =    8408.25 ms /   830 tokens
0.51.026.012 I slot print_timing: id  0 | task 8 |    graphs reused =          5
0.51.026.123 I slot      release: id  0 | task 8 | stop processing: n_tokens = 2760, truncated = 0
`

func TestRequestsReadsOneRequest(t *testing.T) {
	rows, err := Requests(strings.NewReader(oneRequest), "agent", "probe")
	if err != nil {
		t.Fatalf("requests: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	got := rows[0]
	// The prompt is not printed: it is the slot's total at release, less what was
	// generated, plus the one token the server's total runs short by.
	if got.PromptTokens != 2757 || got.IngestedTokens != 826 || got.CachedTokens != 1931 {
		t.Errorf("prompt %d ingested %d cached %d", got.PromptTokens, got.IngestedTokens, got.CachedTokens)
	}
	if got.GeneratedTokens != 4 {
		t.Errorf("generated %d, want 4 — the prompt eval line was read as the generation one",
			got.GeneratedTokens)
	}
	if got.SlotSelection != SelectedByLCP {
		t.Errorf("slot selection %q", got.SlotSelection)
	}
	if got.TaskID != 8 || got.Index != 1 || got.Config != "agent" || got.Session != "probe" {
		t.Errorf("row does not carry what produced it: %+v", got)
	}
	if got.PromptSeconds < 8.14 || got.PromptSeconds > 8.16 {
		t.Errorf("prompt seconds %v, want the 8149.65 ms the line carries", got.PromptSeconds)
	}
}

// A log is normally read while the server is still running, so its last request is
// routinely half-written. Reporting one would put a request that appears to have cost
// nothing into a file whose whole purpose is what requests cost.
func TestRequestsDropsAnUnfinishedRequest(t *testing.T) {
	log := oneRequest + `
0.51.032.845 I slot get_availabl: id  0 | task -1 | selected slot by LRU, t_last = 116510351110
0.51.165.285 I slot launch_slot_: id  0 | task 16 | processing task, is_child = 0
0.57.657.599 I slot print_timing: id  0 | task 16 | prompt eval time =    6246.51 ms /   630 tokens (    9.92 ms per token,   100.86 tokens per second)
`
	rows, err := Requests(strings.NewReader(log), "agent", "probe")
	if err != nil {
		t.Fatalf("requests: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want only the request that finished", len(rows))
	}
}

func TestRequestsRecordsHowTheSlotWasChosen(t *testing.T) {
	log := strings.Replace(oneRequest,
		"selected slot by LCP similarity, f_sim_best = 0.702 (> 0.100 thold), f_keep = 0.998",
		"selected slot by LRU, t_last = 116510351110", 1)
	rows, err := Requests(strings.NewReader(log), "agent", "probe")
	if err != nil {
		t.Fatalf("requests: %v", err)
	}
	if rows[0].SlotSelection != SelectedByLRU {
		t.Errorf("slot selection %q, want %q", rows[0].SlotSelection, SelectedByLRU)
	}
	// The reading this guards against: a request chosen as the least recently used slot
	// still reuses its prefix, because an evicted one is restored from host RAM.
	if rows[0].CachedTokens == 0 {
		t.Error("an LRU-selected request was recorded as reusing nothing")
	}
}

func TestCheckNamesEveryDisagreement(t *testing.T) {
	fromLog := []LogRow{{PromptTokens: 100, IngestedTokens: 10, CachedTokens: 90}}
	agrees := []Row{{PromptTokens: 100, IngestedTokens: 10, CachedTokens: 90}}
	if bad := Check(fromLog, agrees); len(bad) != 0 {
		t.Fatalf("two identical accounts disagreed: %v", bad)
	}

	differs := []Row{{PromptTokens: 100, IngestedTokens: 11, CachedTokens: 89}}
	bad := Check(fromLog, differs)
	if len(bad) != 2 {
		t.Fatalf("got %d disagreements, want one per differing field: %v", len(bad), bad)
	}
	if !strings.Contains(bad[0].String(), "ingested") {
		t.Errorf("disagreement does not name the field: %s", bad[0])
	}
}

// A log holding a different number of requests than the endpoint reported is a
// disagreement in itself: comparing the two accounts pairwise would otherwise pass while
// the instrument silently missed a request.
func TestCheckNoticesADifferentCount(t *testing.T) {
	bad := Check([]LogRow{{PromptTokens: 1}}, []Row{{PromptTokens: 1}, {PromptTokens: 2}})
	if len(bad) != 1 || bad[0].Field != "requests" {
		t.Fatalf("got %v, want one disagreement about the count", bad)
	}
}

// The prompt cache is the one thing about a request that llama-server logs nothing about,
// so the only record of it is the banner scripts/serve.sh writes ahead of the server's
// own output. A row that does not carry it cannot say which cache produced its reuse.
func TestRequestsReadsTheCacheBudgetFromTheBanner(t *testing.T) {
	banner := "serving config/driver-mtp-32k.env: ctx=32768 kv=q8_0/q8_0 cache-ram=%s on 127.0.0.1:8081 via llama-server\n"
	for _, want := range []string{"default", "0", "8192"} {
		rows, err := Requests(strings.NewReader(fmt.Sprintf(banner, want)+oneRequest), "driver", "chain")
		if err != nil {
			t.Fatalf("requests: %v", err)
		}
		if rows[0].CacheRAM != want {
			t.Errorf("cache_ram %q, want %q", rows[0].CacheRAM, want)
		}
	}
}

// A log from a server nobody started through that script has no banner, and a row that
// guessed a budget there would be inventing the number this whole feature exists because
// nobody recorded.
func TestRequestsLeavesTheCacheBudgetEmptyWithoutABanner(t *testing.T) {
	rows, err := Requests(strings.NewReader(oneRequest), "agent", "probe")
	if err != nil {
		t.Fatalf("requests: %v", err)
	}
	if rows[0].CacheRAM != "" {
		t.Errorf("cache_ram %q, want empty", rows[0].CacheRAM)
	}
}
