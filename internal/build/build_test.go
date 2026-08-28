package build

import (
	"runtime/debug"
	"strings"
	"testing"
)

func info(settings ...debug.BuildSetting) *debug.BuildInfo {
	return &debug.BuildInfo{GoVersion: "go1.26.6", Settings: settings}
}

func set(k, v string) debug.BuildSetting { return debug.BuildSetting{Key: k, Value: v} }

// A row is evidence only if somebody can get back to the code that wrote it. A modified
// tree cannot be checked out again, so that fact travels with the revision rather than
// beside it where a reader can miss it.
func TestRevisionSaysWhatCanBeCheckedOutAgain(t *testing.T) {
	for _, tc := range []struct {
		name string
		bi   *debug.BuildInfo
		want string
	}{
		{
			name: "a clean tree",
			bi:   info(set("vcs.revision", "68e89f2a20f5aabbccddeeff00112233"), set("vcs.modified", "false")),
			want: "68e89f2a20f5",
		},
		{
			name: "a tree with uncommitted changes",
			bi:   info(set("vcs.revision", "68e89f2a20f5aabbccddeeff00112233"), set("vcs.modified", "true")),
			want: "68e89f2a20f5+modified",
		},
		{
			// What `go run` produces, which is what `make eval` used to reach for.
			name: "a build that recorded no revision",
			bi:   info(set("vcs.time", "2026-08-28T10:07:37Z")),
			want: Unknown,
		},
		{
			name: "no build info at all",
			bi:   nil,
			want: Unknown,
		},
		{
			name: "a revision shorter than the cut",
			bi:   info(set("vcs.revision", "abc"), set("vcs.modified", "false")),
			want: "abc",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := revisionFrom(tc.bi); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The line a person reads names the binary first, because the revision is the same across
// all six and the binary is what they are asking about.
func TestVersionNamesTheBinaryTheRevisionAndTheToolchain(t *testing.T) {
	got := versionFrom("localcode", info(
		set("vcs.revision", "68e89f2a20f5aabbccddeeff00112233"),
		set("vcs.modified", "true"),
		set("vcs.time", "2026-08-28T10:07:37Z"),
	))
	for _, want := range []string{"localcode", "68e89f2a20f5+modified", "2026-08-28T10:07:37Z", "go1.26.6"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q missing from %q", want, got)
		}
	}
}

// A build with nothing recorded still answers, because a binary that cannot say what it is
// must not also fail to run.
func TestVersionAnswersWithoutBuildInfo(t *testing.T) {
	if got := versionFrom("eval", nil); got != "eval "+Unknown {
		t.Fatalf("got %q", got)
	}
}
