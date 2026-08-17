# Serving (0001). CONFIG is a path to an env file scripts/serve.sh consumes, and
# docs/TECH.md documents it as such — that meaning is shipped and stays.
CONFIG   ?= config/baseline.env

# Scoring (0002). The eval label is LABEL, not CONFIG: it names a run for the
# results file, which is a different thing from the file that launched the server.
# Both meanings collided in the merge, and one silently accepting the other's value
# would mislabel every row it wrote.
LABEL    ?= unlabelled
TASKS    ?= tasks
RESULTS  ?= results/tier1.jsonl
N        ?= 1
THINKING ?=

.PHONY: build check test smoke serve eval report

## build: compile everything
build:
	@go build ./...

## check: the ship gate — the scorer's own correctness, then a live endpoint
check: test smoke

## test: unit tests only; needs no server
test:
	@go test ./...

## smoke: assert a served endpoint answers and round-trips a tool call
smoke:
	@scripts/smoke.sh

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
