// Package scripts holds no Go code. It exists so the shell in this directory can be
// tested by the same `go test -race ./...` the gate already runs: a test here puts stub
// executables on PATH and asserts what a script did and did not shell out to, which is
// the only way to exercise a `sudo sysctl` without running one.
package scripts
