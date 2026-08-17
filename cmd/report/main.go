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
	outcomes        map[eval.Outcome]int
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
	if err := fs.Parse(args); err != nil {
		return err
	}

	f, err := os.Open(*path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	byConfig := map[string]map[string]*agg{}
	served := map[string]string{}

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
		if byConfig[r.Config] == nil {
			byConfig[r.Config] = map[string]*agg{}
		}
		key := r.Thinking
		if key == "" {
			key = "(default)"
		}
		a := byConfig[r.Config][key]
		if a == nil {
			a = &agg{outcomes: map[eval.Outcome]int{}}
			byConfig[r.Config][key] = a
		}
		a.total++
		a.outcomes[r.Outcome]++
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
		served[r.Config] = fmt.Sprintf("ctx=%d model=%s", r.ServedNCtx, r.ServedModel)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(byConfig) == 0 {
		_, _ = fmt.Fprintln(stdout, "no rows matched")
		return nil
	}

	for _, cfg := range sortedKeys(byConfig) {
		_, _ = fmt.Fprintf(stdout, "\n%s  [%s]\n", cfg, served[cfg])
		_, _ = fmt.Fprintf(stdout, "  %-12s %-8s %-16s %-18s %-18s %s\n",
			"thinking", "pass", "toolcall valid", "gen tok/s", "completion tok", "wall s")
		for _, th := range sortedKeys(byConfig[cfg]) {
			a := byConfig[cfg][th]
			tc := "n/a"
			if a.tcSeen > 0 {
				tc = fmt.Sprintf("%d/%d", a.tcValid, a.tcSeen)
			}
			_, _ = fmt.Fprintf(stdout, "  %-12s %-8s %-16s %-18s %-18s %s\n",
				th, fmt.Sprintf("%d/%d", a.pass, a.total), tc,
				rangeF(a.gen), rangeI(a.completion), rangeF(a.wall))
			if s := failSummary(a.outcomes); s != "" {
				_, _ = fmt.Fprintf(stdout, "  %-12s %s\n", "", s)
			}
		}
	}
	_, _ = fmt.Fprintln(stdout)
	return nil
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
