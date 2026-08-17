CONFIG ?= config/baseline.env

.PHONY: serve check

## serve: start llama-server from CONFIG (default config/baseline.env)
serve:
	@scripts/serve.sh $(CONFIG)

## check: assert a served endpoint answers and round-trips a tool call
check:
	@scripts/smoke.sh
