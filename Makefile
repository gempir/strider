.PHONY: *

STRIDER ?= ./strider
CORPUS_FLAGS ?=

build:
	CGO_ENABLED=0 go build -trimpath -o strider ./cmd/strider

install: build
	mv ./strider ~/.local/bin/strider

test:
	go test ./...

corpus-check: build
	go run ./scripts/corpus --mode check --strider "$(STRIDER)" $(CORPUS_FLAGS)

corpus-update: build
	go run ./scripts/corpus --mode update --strider "$(STRIDER)" \
		--html docs/public/benchmark-report/index.html $(CORPUS_FLAGS)
