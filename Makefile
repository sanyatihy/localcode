CONFIG  ?= unlabelled
TASKS   ?= tasks
RESULTS ?= results/tier1.jsonl
N       ?= 1
THINKING ?=

.PHONY: check eval report build

## build: compile everything
build:
	@go build ./...

## check: unit tests — the scorer's own correctness, no server needed
check:
	@go test ./...

## eval: run the tier-1 suite N times against CONFIG, then summarise
eval:
	@go run ./cmd/eval -tasks $(TASKS) -n $(N) -config "$(CONFIG)" \
		-results $(RESULTS) $(if $(THINKING),-thinking $(THINKING),) $(EVALFLAGS) || true
	@$(MAKE) --no-print-directory report

## report: summarise the results file
report:
	@go run ./cmd/report -results $(RESULTS)
