.PHONY: *

STRIDER ?= ./strider
CORPUS_FLAGS ?=
CHECKSCAFFOLD_FLAGS ?=

build:
	CGO_ENABLED=0 go build -trimpath -o strider ./cmd/strider

install: build
	mv ./strider ~/.local/bin/strider

verify: check test vet unused-check

test:
	go test ./...

golden-update:
	STRIDER_UPDATE_GOLDEN=1 go test ./internal/checks/... ./internal/report

check-update: golden-update
	cd docs && bun run generate:checks

check-scaffold:
	go run ./scripts/checkscaffold $(CHECKSCAFFOLD_FLAGS)

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
		--homepage-stats docs/src/generated/kubernetes-benchmark.json $(CORPUS_FLAGS)
