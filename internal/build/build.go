// Package build reports which build of this repository is running.
//
// VISION says a number nobody can regenerate is not evidence, and a row that cannot name
// the driver that wrote it is one: 0043 moved the budget arithmetic every session runs
// under, and rows either side of it are otherwise indistinguishable.
//
// Nothing here is stamped by the Makefile. `go build` records the revision, the commit time
// and whether the tree was modified into the binary's own build info, so the only thing to
// get wrong is reading it.
package build

import (
	"runtime/debug"
	"strings"
)

// Unknown is what a build that recorded no revision answers. `go run` is the case that
// matters: it stamps no VCS settings at all, and `make eval` uses it — so a row says its
// build could not be known rather than carrying a blank that reads as one.
const Unknown = "unknown"

// modifiedSuffix marks a build whose tree carried uncommitted changes. Such a build cannot
// be checked out again, which is exactly what makes a row from it unreproducible, so it is
// part of the revision rather than a field beside it that a reader can miss.
const modifiedSuffix = "+modified"

// short is how much of a revision a row carries. Enough to check out, short enough to read
// beside the other columns.
const short = 12

// Revision is the commit this binary was built from, suffixed `+modified` when the tree it
// was built from was not clean, and Unknown when the build recorded no revision.
func Revision() string {
	bi, _ := debug.ReadBuildInfo()
	return revisionFrom(bi)
}

func revisionFrom(bi *debug.BuildInfo) string {
	if bi == nil {
		return Unknown
	}
	rev, modified := "", false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if rev == "" {
		return Unknown
	}
	if len(rev) > short {
		rev = rev[:short]
	}
	if modified {
		return rev + modifiedSuffix
	}
	return rev
}

// Version is the line a binary prints for -version: what it is, which commit it came from,
// when that commit was made, and which toolchain built it.
func Version(name string) string {
	bi, _ := debug.ReadBuildInfo()
	return versionFrom(name, bi)
}

func versionFrom(name string, bi *debug.BuildInfo) string {
	parts := []string{name, revisionFrom(bi)}
	if bi != nil {
		for _, s := range bi.Settings {
			if s.Key == "vcs.time" && s.Value != "" {
				parts = append(parts, s.Value)
			}
		}
		if bi.GoVersion != "" {
			parts = append(parts, bi.GoVersion)
		}
	}
	return strings.Join(parts, " ")
}
