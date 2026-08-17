package eval

import (
	"os"
	"path/filepath"
	"sort"
)

// DiscoverTasks finds fixtures under dir. Two shapes are supported because two are
// needed: a bare <name>.json for tasks that are only a prompt, and <name>/task.json
// for tasks carrying fixture files beside them, which patch tasks require.
func DiscoverTasks(dir string) ([]string, error) {
	var out []string

	flat, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	out = append(out, flat...)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), "task.json")
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}
