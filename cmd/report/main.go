// Command report summarises a results file: pass rate and per-metric spread,
// grouped by config and thinking mode.
//
// Spread is min-max rather than a standard deviation. Repeat counts here are small
// — three passes of the suite is already a multi-hour job on this hardware — and a
// standard deviation over three samples implies precision the data does not have,
// while a range shows exactly how far apart the runs fell.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sanyatihy/localcode/internal/eval"
)

type agg struct {
	pass, total     int
	tcValid, tcSeen int
	gen, wall       []float64
	completion      []int
	prompt, cached  []int
	turns           []int
	outcomes        map[eval.Outcome]int

	// A swapped run is void rather than slow, and a run that measured no memory cannot
	// claim to be clean. Both are counted so the summary can say so instead of folding
	// them into an average that looks fine.
	swapped, unmeasured int
	maxSwap             float64

	// A harness the desk profile excluded never ran, so it is held apart from the pass
	// rate and printed with the reason. Folded in, it would read as a harness that
	// failed everything; left out, its absence from the table would read as an
	// oversight rather than as the constraint it is.
	inadmissible int
	whyExcluded  string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "report: %v\n", err)
		os.Exit(2)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("results", "results/tier1.jsonl", "results file to summarise")
	only := fs.String("config", "", "summarise only this config label")
	baseline := fs.String("baseline", "", "harness to report the others against, e.g. claude-code")
	if err := fs.Parse(args); err != nil {
		return err
	}

	f, err := os.Open(*path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// Rows that carry a session are the ones a ratio may be taken over, kept aside so
	// the paired section can divide them after the summary. Everything else about the
	// report is unchanged by their presence.
	paired := map[string]map[string]*pairAgg{}

	byConfig := map[string]map[string]*agg{}
	served := map[string]string{}
	// Which groups compare harnesses rather than thinking modes, so the first column
	// can be named after what is in it.
	byHarness := map[string]bool{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var r eval.Row
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			_, _ = fmt.Fprintf(stderr, "report: skipping unparseable row: %v\n", err)
			continue
		}
		if *only != "" && r.Config != *only {
			continue
		}
		// The desk profile is part of the group, not a label on it: two profiles cap
		// the context differently, so their rows are not comparable and a harness may
		// be admissible under only one of them. Filtering stays on the config label,
		// which is what a human types.
		group := r.Config
		if r.Profile != "" {
			group += " · " + r.Profile
		}
		if byConfig[group] == nil {
			byConfig[group] = map[string]*agg{}
		}
		// Tier-2 rows are one harness each at one serving config, and the thinking
		// toggle on that path is the harness's own business and never set — so the
		// harness is what separates them, exactly as thinking separates tier-1 rows.
		key := r.Thinking
		if r.Harness != "" {
			key = r.Harness
			byHarness[group] = true
		}
		if key == "" {
			key = "(default)"
		}
		a := byConfig[group][key]
		if a == nil {
			a = &agg{outcomes: map[eval.Outcome]int{}}
			byConfig[group][key] = a
		}

		// A tier-2 row records no served config — it drives a harness that builds its
		// own requests — and printing ctx=0 there would read as a server serving no
		// context rather than as a figure nobody took.
		props := "served config unrecorded"
		if r.ServedNCtx > 0 {
			props = fmt.Sprintf("ctx=%d model=%s", r.ServedNCtx, r.ServedModel)
		}
		served[group] = props

		if r.Session != "" {
			if paired[r.Session] == nil {
				paired[r.Session] = map[string]*pairAgg{}
			}
			pa := paired[r.Session][r.Config]
			if pa == nil {
				pa = &pairAgg{}
				paired[r.Session][r.Config] = pa
			}
			pa.add(r)
		}

		if r.Outcome == eval.Inadmissible {
			a.inadmissible++
			a.whyExcluded = r.Detail
			continue
		}
		a.total++
		a.outcomes[r.Outcome]++
		// 20 MB of slack: macOS moves swap around by a few MB without the run causing it,
		// and flagging that as contamination would cry wolf on every clean sweep.
		switch {
		case !r.MemMeasured:
			a.unmeasured++
		case r.SwapDeltaMB > 20:
			a.swapped++
			if r.SwapDeltaMB > a.maxSwap {
				a.maxSwap = r.SwapDeltaMB
			}
		}
		if r.Outcome == eval.Pass {
			a.pass++
		}
		if valid, applies := r.ToolCallValid(); applies {
			a.tcSeen++
			if valid {
				a.tcValid++
			}
		}
		a.gen = append(a.gen, r.GenPerSecond)
		a.wall = append(a.wall, r.WallSeconds)
		a.completion = append(a.completion, r.CompletionTokens)
		a.prompt = append(a.prompt, r.PromptTokens)
		a.cached = append(a.cached, r.CachedTokens)
		a.turns = append(a.turns, r.Turns)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(byConfig) == 0 {
		_, _ = fmt.Fprintln(stdout, "no rows matched")
		return nil
	}

	for _, cfg := range sortedKeys(byConfig) {
		harnesses := byHarness[cfg]
		_, _ = fmt.Fprintf(stdout, "\n%s  [%s]\n", cfg, served[cfg])
		// A tier-2 group answers different questions from a tier-1 one: what a whole
		// task cost, and in how many turns. Its columns say so rather than leaving
		// "toolcall valid" reading n/a beside a column of zeroes.
		if harnesses {
			_, _ = fmt.Fprintf(stdout, "  %-12s %-8s %-14s %-22s %-22s %-20s %s\n",
				"harness", "pass", "turns", "prompt tok", "cached tok", "predicted tok", "wall s")
		} else {
			_, _ = fmt.Fprintf(stdout, "  %-12s %-8s %-16s %-18s %-18s %s\n",
				"thinking", "pass", "toolcall valid", "gen tok/s", "completion tok", "wall s")
		}
		for _, th := range sortedKeys(byConfig[cfg]) {
			a := byConfig[cfg][th]
			tc := "n/a"
			if a.tcSeen > 0 {
				tc = fmt.Sprintf("%d/%d", a.tcValid, a.tcSeen)
			}
			// A group with nothing but excluded rows has no pass rate, and "0/0"
			// there would read as a harness that failed every task it was given.
			pass := "-"
			if a.total > 0 {
				pass = fmt.Sprintf("%d/%d", a.pass, a.total)
			}
			if harnesses {
				name := th
				if th == *baseline {
					name += "*"
				}
				_, _ = fmt.Fprintf(stdout, "  %-12s %-8s %-14s %-22s %-22s %-20s %s\n",
					name, pass, rangeI(a.turns), rangeI(a.prompt), rangeI(a.cached),
					rangeI(a.completion), rangeF(a.wall))
				// A challenger that ties has lost — switching costs something — so the
				// numbers that decide are the ratios, not the absolutes beside them.
				if base := byConfig[cfg][*baseline]; base != nil && th != *baseline {
					_, _ = fmt.Fprintf(stdout, "  %-12s vs %s: %s\n", "", *baseline, versus(a, base))
				}
			} else {
				_, _ = fmt.Fprintf(stdout, "  %-12s %-8s %-16s %-18s %-18s %s\n",
					th, pass, tc,
					rangeF(a.gen), rangeI(a.completion), rangeF(a.wall))
			}
			if a.inadmissible > 0 {
				_, _ = fmt.Fprintf(stdout, "  %-12s NOT ADMISSIBLE: %s\n", "", a.whyExcluded)
			}
			if s := failSummary(a.outcomes); s != "" {
				_, _ = fmt.Fprintf(stdout, "  %-12s %s\n", "", s)
			}
			// A run whose swap grew was measuring the pager. The vision calls that void
			// rather than slow, so it is named here instead of being averaged into the
			// timings above, which is what would make it invisible.
			if a.swapped > 0 {
				_, _ = fmt.Fprintf(stdout, "  %-12s VOID: %d run(s) swapped (max +%.0f MB) — timings measure paging\n",
					"", a.swapped, a.maxSwap)
			}
			if a.unmeasured > 0 {
				_, _ = fmt.Fprintf(stdout, "  %-12s %d run(s) recorded no memory; cleanliness is unverified\n",
					"", a.unmeasured)
			}
		}
	}
	reportPaired(stdout, paired, *baseline)
	_, _ = fmt.Fprintln(stdout)
	return nil
}

// versus reads a challenger against the baseline. Pass rate is stated as a difference —
// two more tasks passed is two more tasks — and everything else as a ratio, because what
// the comparison turns on is proportion: half the tokens is the finding, not 1,600 fewer.
func versus(a, base *agg) string {
	// Tasks, when both were asked the same number of them; otherwise the rate, since a
	// harness the profile excluded from some of them has a different denominator and a
	// count would be comparing two different questions.
	quality := fmt.Sprintf("pass %+d", a.pass-base.pass)
	if a.total != base.total {
		quality = fmt.Sprintf("pass %+.0f pp", 100*(rate(a.pass, a.total)-rate(base.pass, base.total)))
	}
	parts := []string{quality}
	for _, m := range []struct {
		name string
		xs   []int
	}{
		{"turns", a.turns}, {"prompt", a.prompt}, {"cached", a.cached}, {"out", a.completion},
	} {
		parts = append(parts, ratio(m.name, meanI(m.xs), meanI(baseOf(base, m.name))))
	}
	parts = append(parts, ratio("wall", meanF(a.wall), meanF(base.wall)))
	return strings.Join(parts, "  ")
}

// pairAgg is one config's decode measurements inside one session.
type pairAgg struct {
	secPerToken []float64
	tau         []float64
	swapped     int
	unmeasured  int
}

func (p *pairAgg) add(r eval.Row) {
	if r.AcceptanceMeasured {
		p.tau = append(p.tau, r.AcceptanceLength)
	}
	if !r.DecodeMeasured || r.CompletionTokens <= 0 || r.DecodeSeconds <= 0 {
		p.unmeasured++
		return
	}
	// A run whose swap grew measured the pager, which the vision calls void rather than
	// slow. Counted and named, never averaged in.
	if r.MemMeasured && r.SwapDeltaMB > 0 {
		p.swapped++
		return
	}
	p.secPerToken = append(p.secPerToken, r.DecodeSeconds/float64(r.CompletionTokens))
}

// reportPaired prints the decode ratio for each session, against the baseline named on
// the command line or, failing that, against the config that reported no acceptance —
// a run that drafted nothing is the run with the mechanism off, and that is what a
// candidate is divided by.
//
// The ratio is of seconds per token and not of wall: a speculative decoder moves decode
// and cannot move prefill, and on a deep prompt wall is nearly all prefill. Sessions
// exist so the two sides were measured back to back on one machine state; dividing
// across sessions would measure host drift as well as the change.
func reportPaired(stdout *os.File, paired map[string]map[string]*pairAgg, baseline string) {
	if len(paired) == 0 {
		return
	}
	_, _ = fmt.Fprintf(stdout, "\npaired decode ratio\n")
	for _, session := range sortedKeys(paired) {
		configs := paired[session]
		base := baseline
		if _, ok := configs[base]; !ok {
			base = ""
			for _, name := range sortedKeys(configs) {
				if len(configs[name].tau) == 0 {
					if base != "" {
						base = "" // two candidates for the reference is not a guess to make
						break
					}
					base = name
				}
			}
		}
		_, _ = fmt.Fprintf(stdout, "  %s\n", session)
		if base == "" {
			_, _ = fmt.Fprintf(stdout, "    no baseline: name one with -baseline, or measure one config without speculation\n")
			continue
		}
		bm := meanF(configs[base].secPerToken)
		for _, name := range sortedKeys(configs) {
			c := configs[name]
			note := ""
			if c.swapped > 0 {
				note += fmt.Sprintf("  (%d void: swap grew)", c.swapped)
			}
			if c.unmeasured > 0 {
				note += fmt.Sprintf("  (%d unmeasured)", c.unmeasured)
			}
			tau := "tau unavailable"
			if len(c.tau) > 0 {
				tau = fmt.Sprintf("tau=%.2f", meanF(c.tau))
			}
			if name == base {
				_, _ = fmt.Fprintf(stdout, "    %-28s %6.4f s/token over %d  %s  (baseline)%s\n",
					name, bm, len(c.secPerToken), tau, note)
				continue
			}
			m := meanF(c.secPerToken)
			r := "ratio unavailable"
			if m > 0 && bm > 0 {
				r = fmt.Sprintf("%.2fx", bm/m)
			}
			_, _ = fmt.Fprintf(stdout, "    %-28s %6.4f s/token over %d  %s  %s%s\n",
				name, m, len(c.secPerToken), tau, r, note)
		}
	}
}

func rate(pass, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(pass) / float64(total)
}

func baseOf(base *agg, name string) []int {
	switch name {
	case "turns":
		return base.turns
	case "prompt":
		return base.prompt
	case "cached":
		return base.cached
	default:
		return base.completion
	}
}

// ratio says "half" as ×0.50 rather than −50%, which reads the same for a doubling and a
// halving. A baseline of zero has no ratio and says so instead of dividing.
func ratio(name string, got, want float64) string {
	if want == 0 {
		return name + " n/a"
	}
	return fmt.Sprintf("%s ×%.2f", name, got/want)
}

func meanI(xs []int) float64 {
	f := make([]float64, len(xs))
	for i, x := range xs {
		f[i] = float64(x)
	}
	return meanF(f)
}

func meanF(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func failSummary(m map[eval.Outcome]int) string {
	var parts []string
	for _, k := range sortedOutcomes(m) {
		if k != eval.Pass {
			parts = append(parts, fmt.Sprintf("%s×%d", k, m[k]))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "failures: " + strings.Join(parts, " ")
}

func rangeF(xs []float64) string {
	if len(xs) == 0 {
		return "-"
	}
	lo, hi, sum := xs[0], xs[0], 0.0
	for _, x := range xs {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
		sum += x
	}
	mean := sum / float64(len(xs))
	if len(xs) == 1 {
		return fmt.Sprintf("%.1f", mean)
	}
	return fmt.Sprintf("%.1f (%.1f-%.1f)", mean, lo, hi)
}

func rangeI(xs []int) string {
	f := make([]float64, len(xs))
	for i, x := range xs {
		f[i] = float64(x)
	}
	return rangeF(f)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedOutcomes(m map[eval.Outcome]int) []eval.Outcome {
	out := make([]eval.Outcome, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
