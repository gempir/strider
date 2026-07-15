STRIDER ?= ./strider
WILDS_DIR ?= .wilds
WILDS_BASELINES ?= testdata/wilds/baselines
TIMINGS_DIR ?= target/timings
CURATED_MAX_SECONDS ?= 1.0
WILDS_FMT_MAX_SECONDS ?= 2.0
WILDS_LINT_MAX_SECONDS ?= 2.0
WILDS_ANALYZE_MAX_SECONDS ?= 10.0
STRIDER_ANALYZE_ARGS ?=

# Format: name,repository,commit. Pin commits so baseline checks are repeatable.
WILDS_PROJECTS ?= \
	go-twitch-irc,https://github.com/gempir/go-twitch-irc.git,2e3318729badccf5dee9362ff223b820e1db8774 \
	task,https://github.com/go-task/task.git,81c4291803169106d53775e56e9d2e90d2a64f42 \
	pajbot2,https://github.com/pajbot/pajbot2.git,ba1a68dd6f31c62dbe886c4bf843b92d6609644d \
	helix,https://github.com/nicklaw5/helix.git,15cffe632969bd9f5b99a19fa2fee8e55a13ce2f \
	tailscale,https://github.com/tailscale/tailscale.git,168b20d3b42088aafa30e73dd57e8590ad8d5fbd

.PHONY: build test require-strider wilds wilds-all wilds-analyze wilds-clone wilds-check wilds-accept
.PHONY: test-projects test-projects-clone

build:
	CGO_ENABLED=0 go build -trimpath -o strider ./cmd/strider

# Deterministic, curated formatter and linter expectations. This uses only the
# existing Strider binary; it does not invoke Go.
test: require-strider
	@TIMINGS_FILE="$(TIMINGS_DIR)/curated.tsv" \
		CURATED_MAX_SECONDS="$(CURATED_MAX_SECONDS)" \
		./scripts/test.sh "$(STRIDER)"

# Exercise valid real-world code. Findings and formatting differences (exit 1)
# are observations; crashes and processing errors (exit 2+) fail the smoke run.
wilds: require-strider wilds-clone
	@TIMINGS_FILE="$(TIMINGS_DIR)/wilds-smoke.tsv" \
		WILDS_FMT_MAX_SECONDS="$(WILDS_FMT_MAX_SECONDS)" \
		WILDS_LINT_MAX_SECONDS="$(WILDS_LINT_MAX_SECONDS)" \
		./scripts/wilds.sh smoke "$(STRIDER)" "$(WILDS_DIR)" "$(WILDS_BASELINES)" $(WILDS_PROJECTS)

wilds-all: require-strider wilds-clone
	@TIMINGS_FILE="$(TIMINGS_DIR)/wilds-all.tsv" \
		STRIDER_LINT_ARGS="--all-rules" \
		STRIDER_SKIP_FORMAT="1" \
		STRIDER_SUITE_NAME="wilds-all" \
		WILDS_FMT_MAX_SECONDS="$(WILDS_FMT_MAX_SECONDS)" \
		WILDS_LINT_MAX_SECONDS="$(WILDS_LINT_MAX_SECONDS)" \
		./scripts/wilds.sh smoke "$(STRIDER)" "$(WILDS_DIR)" "$(WILDS_BASELINES)" $(WILDS_PROJECTS)

wilds-analyze: require-strider wilds-clone
	@TIMINGS_FILE="$(TIMINGS_DIR)/wilds-analyze.tsv" \
		STRIDER_ANALYZE_ARGS="$(STRIDER_ANALYZE_ARGS)" \
		STRIDER_RUN_ANALYZE="1" \
		STRIDER_SKIP_FORMAT="1" \
		STRIDER_SKIP_LINT="1" \
		STRIDER_SUITE_NAME="wilds-analyze" \
		WILDS_ANALYZE_MAX_SECONDS="$(WILDS_ANALYZE_MAX_SECONDS)" \
		./scripts/wilds.sh smoke "$(STRIDER)" "$(WILDS_DIR)" "$(WILDS_BASELINES)" $(WILDS_PROJECTS)

# Compare all output and exit codes with the reviewed, pinned behavior.
wilds-check: require-strider wilds-clone
	@TIMINGS_FILE="$(TIMINGS_DIR)/wilds-check.tsv" \
		WILDS_FMT_MAX_SECONDS="$(WILDS_FMT_MAX_SECONDS)" \
		WILDS_LINT_MAX_SECONDS="$(WILDS_LINT_MAX_SECONDS)" \
		./scripts/wilds.sh check "$(STRIDER)" "$(WILDS_DIR)" "$(WILDS_BASELINES)" $(WILDS_PROJECTS)

# Explicitly accept the current pinned behavior after reviewing the changes.
wilds-accept: require-strider wilds-clone
	@TIMINGS_FILE="$(TIMINGS_DIR)/wilds-accept.tsv" \
		WILDS_FMT_MAX_SECONDS="$(WILDS_FMT_MAX_SECONDS)" \
		WILDS_LINT_MAX_SECONDS="$(WILDS_LINT_MAX_SECONDS)" \
		./scripts/wilds.sh accept "$(STRIDER)" "$(WILDS_DIR)" "$(WILDS_BASELINES)" $(WILDS_PROJECTS)

wilds-clone:
	@./scripts/wilds.sh clone - "$(WILDS_DIR)" "$(WILDS_BASELINES)" $(WILDS_PROJECTS)

require-strider:
	@test -x "$(STRIDER)" || { \
		echo "error: $(STRIDER) is not executable; build or download strider first" >&2; \
		exit 2; \
	}

# Backwards-compatible names for the original external-project harness.
test-projects: wilds

test-projects-clone: wilds-clone
