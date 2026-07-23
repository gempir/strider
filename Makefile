.PHONY: *

STRIDER ?= ./strider
CORPUS_FLAGS ?=

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
	go run ./scripts/catalogreview

vet:
	go vet ./...

check:
	go run cmd/strider/main.go check

unused-check:
	@output="$$(go run golang.org/x/tools/cmd/deadcode@v0.48.0 -test ./...)"; \
	status="$$?"; \
	if [ "$$status" -ne 0 ]; then \
		exit "$$status"; \
	fi; \
	if [ -n "$$output" ]; then \
		printf '%s\n' "$$output"; \
		exit 1; \
	fi

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

catalog-review:
	go run ./scripts/catalogreview
