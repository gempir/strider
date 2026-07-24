.PHONY: *

STRIDER ?= ./strider
CORPUS_FLAGS ?=
CORPUS_UPDATE_FLAGS ?= --samples 1 --warmups 0 --scheduler-modes fixed --strider-cache-modes cold,warm --go-cache-modes warm

build:
	CGO_ENABLED=0 go build -trimpath -o strider ./cmd/strider

install: build
	mv ./strider ~/.local/bin/strider

verify: check test vet

test:
	go test ./...

golden-update:
	STRIDER_UPDATE_GOLDEN=1 go test ./internal/checks/... ./internal/report

check-update: golden-update
	cd docs && bun run generate:checks

vet:
	go vet ./...

check:
	go run cmd/strider/main.go check

dependency-check:
	go mod verify
	go mod tidy -diff

corpus-check: build
	go run ./scripts/corpus --mode check --strider "$(STRIDER)" $(CORPUS_FLAGS)

corpus-update: build
	go run ./scripts/corpus --mode update --strider "$(STRIDER)" \
		--html target/corpus/index.html \
		--project-html docs/public/benchmark-report/projects \
		--homepage-stats docs/src/generated/sftpgo-benchmark.json \
		$(CORPUS_UPDATE_FLAGS) $(CORPUS_FLAGS)
