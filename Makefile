# CONFIG is a path to an env file scripts/serve.sh consumes. The default is the config
# the measurements settled on, so `make serve` serves what the project concluded.
CONFIG   ?= config/tuned.env

# Scoring (0002). The eval label is LABEL, not CONFIG: it names a run for the
# results file, which is a different thing from the file that launched the server.
LABEL    ?= unlabelled
TASKS    ?= tasks
RESULTS  ?= results/tier1.jsonl
N        ?= 1

# The thinking toggle carries its own sampling, and cmd/eval refuses one without the
# other: at a single fixed sampling a toggle sweep measures the pair, not the toggle.
# So these two move together — `make eval THINKING=off SAMPLING=nonthinking`.
THINKING ?=
SAMPLING ?=

.PHONY: help build check fmt vet lint test smoke verify serve eval report

## help: list these targets
help:
	@grep -hE '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build: compile everything
build:
	@go build ./...

## check: the offline gate — gofmt, vet, lint, tests under the race detector
# What CI runs, so it must need no server and no model weights.
check: fmt vet lint test

fmt:
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || { echo "gofmt: files need formatting"; exit 1; }

vet:
	@go vet ./...

## lint: golangci-lint, pinned by .golangci.yml; skipped loudly when absent
# An `if`, never `command -v ... && run || echo`: in that form a lint *failure* also
# takes the `||` branch, so findings print as "SKIPPED" and the gate exits 0.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "lint: golangci-lint not installed, SKIPPED (CI will still run it)"; \
	fi

test:
	@go test -race ./...

## smoke: assert a running endpoint answers and round-trips a tool call
# Apart from check because it needs a live llama-server, which CI has not.
smoke:
	@scripts/smoke.sh

## verify: the pre-ship gate — offline checks plus a live endpoint
verify: check smoke

## serve: start llama-server from CONFIG
serve:
	@scripts/serve.sh $(CONFIG)

## eval: run the tier-1 suite N times under LABEL, then summarise
eval:
	@go run ./cmd/eval -tasks $(TASKS) -n $(N) -config "$(LABEL)" \
		-results $(RESULTS) $(if $(THINKING),-thinking $(THINKING),) \
		$(if $(SAMPLING),-sampling-profile $(SAMPLING),) $(EVALFLAGS) || true
	@$(MAKE) --no-print-directory report

## report: summarise the results file
report:
	@go run ./cmd/report -results $(RESULTS)
