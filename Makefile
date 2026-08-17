# Serving (0001). CONFIG is a path to an env file scripts/serve.sh consumes, and
# docs/TECH.md documents it as such — that meaning is shipped and stays.
CONFIG   ?= config/baseline.env

# Scoring (0002). The eval label is LABEL, not CONFIG: it names a run for the
# results file, which is a different thing from the file that launched the server.
LABEL    ?= unlabelled
TASKS    ?= tasks
RESULTS  ?= results/tier1.jsonl
N        ?= 1
THINKING ?=

.PHONY: build check fmt vet test smoke verify serve eval report

## build: compile everything
build:
	@go build ./...

## check: the offline gate — formatting, vet, and tests under the race detector.
## Runs in CI, so it must need no server and no model weights.
check: fmt vet test

fmt:
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || { echo "gofmt: files need formatting"; exit 1; }

vet:
	@go vet ./...

test:
	@go test -race ./...

## smoke: assert a *served* endpoint answers and round-trips a tool call.
## Separate from check because it needs a running llama-server, which CI has not.
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
		-results $(RESULTS) $(if $(THINKING),-thinking $(THINKING),) $(EVALFLAGS) || true
	@$(MAKE) --no-print-directory report

## report: summarise the results file
report:
	@go run ./cmd/report -results $(RESULTS)
