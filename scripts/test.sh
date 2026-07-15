#!/bin/sh

set -u
export LC_ALL=C

if test "$#" -ne 1; then
	echo "error: usage: test.sh STRIDER" >&2
	exit 2
fi

root=$(pwd -P)
strider=$1
case "$strider" in
/*) ;;
*) strider="$root/${strider#./}" ;;
esac
test -x "$strider" || {
	echo "error: $strider is not executable" >&2
	exit 2
}

temporary=$(mktemp -d "${TMPDIR:-/tmp}/strider-tests.XXXXXX") || exit 2
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
failed=0
total_seconds=0
max_seconds=${CURATED_MAX_SECONDS:-1.0}
timings_file=${TIMINGS_FILE:-target/timings/curated.tsv}
mkdir -p "$(dirname "$timings_file")" || exit 2
printf 'suite\tproject\toperation\tseconds\tbudget_seconds\tbudget_result\n' > "$timings_file"

if test -n "${GITHUB_STEP_SUMMARY:-}"; then
	{
		echo "### Curated suite timings"
		echo
		echo "| Check | Time | Budget | Result |"
		echo "| --- | ---: | ---: | --- |"
	} >> "$GITHUB_STEP_SUMMARY"
fi

run_timed() {
	stdout_file=$1
	stderr_file=$2
	time_file=$3
	shift 3
	{ time -p "$@" > "$stdout_file" 2> "$stderr_file"; } 2> "$time_file"
}

record_timing() {
	test_name=$1
	time_file=$2
	seconds=$(awk '$1 == "real" { print $2; exit }' "$time_file")
	test -n "$seconds" || {
		echo "error: no timing recorded for $test_name" >&2
		failed=1
		seconds=0
	}
	total_seconds=$(awk -v total="$total_seconds" -v current="$seconds" \
		'BEGIN { printf "%.2f", total + current }')
	if awk -v actual="$seconds" -v budget="$max_seconds" \
		'BEGIN { exit !(actual <= budget) }'
	then
		budget_result=PASS
	else
		budget_result=FAIL
		echo "error: $test_name took ${seconds}s; budget is ${max_seconds}s" >&2
		failed=1
	fi
	printf 'Timing: %s: %ss (budget %ss) [%s]\n' \
		"$test_name" "$seconds" "$max_seconds" "$budget_result"
	printf 'curated\t-\t%s\t%s\t%s\t%s\n' \
		"$test_name" "$seconds" "$max_seconds" "$budget_result" >> "$timings_file"
	if test -n "${GITHUB_STEP_SUMMARY:-}"; then
		printf '| %s | %ss | %ss | %s |\n' \
			"$test_name" "$seconds" "$max_seconds" "$budget_result" \
			>> "$GITHUB_STEP_SUMMARY"
	fi
}

expect_status() {
	test_name=$1
	expected=$2
	actual=$3
	if test "$actual" -ne "$expected"; then
		echo "error: $test_name exited $actual; expected $expected" >&2
		failed=1
	fi
}

expect_file() {
	test_name=$1
	expected=$2
	actual=$3
	if ! diff -u "$expected" "$actual"; then
		echo "error: $test_name output changed" >&2
		failed=1
	fi
}

expect_empty() {
	test_name=$1
	actual=$2
	if test -s "$actual"; then
		echo "error: $test_name produced unexpected output:" >&2
		cat "$actual" >&2
		failed=1
	fi
}

format_dir=testdata/cases/format
lint_dir=testdata/cases/lint
analyze_dir=testdata/cases/analyze

run_timed "$temporary/basic.go" "$temporary/format.stderr" "$temporary/format.time" \
	"$strider" fmt --stdin --stdin-filename basic.go \
	< "$format_dir/basic.input.go"

code=$?
record_timing "formatter golden" "$temporary/format.time"
expect_status "formatter golden test" 0 "$code"
expect_file "formatter golden test" "$format_dir/basic.expected.go" "$temporary/basic.go"
expect_empty "formatter golden test stderr" "$temporary/format.stderr"

run_timed "$temporary/idempotent.go" "$temporary/idempotent.stderr" "$temporary/idempotent.time" \
	"$strider" fmt --stdin --stdin-filename basic.go \
	< "$format_dir/basic.expected.go"

code=$?
record_timing "formatter idempotence" "$temporary/idempotent.time"
expect_status "formatter idempotence test" 0 "$code"
expect_file "formatter idempotence test" "$format_dir/basic.expected.go" "$temporary/idempotent.go"
expect_empty "formatter idempotence test stderr" "$temporary/idempotent.stderr"

run_timed "$temporary/lint-findings.stdout" "$temporary/lint-findings.stderr" \
	"$temporary/lint-findings.time" \
	"$strider" lint --only no-init,no-package-var "$lint_dir/findings.go"
code=$?
record_timing "lint true-positive" "$temporary/lint-findings.time"
expect_status "lint true-positive test" 1 "$code"
expect_file "lint true-positive test" "$lint_dir/findings.expected" "$temporary/lint-findings.stdout"
expect_empty "lint true-positive test stderr" "$temporary/lint-findings.stderr"

run_timed "$temporary/lint-clean.stdout" "$temporary/lint-clean.stderr" \
	"$temporary/lint-clean.time" \
	"$strider" lint --only no-init,no-package-var "$lint_dir/clean.go"
code=$?
record_timing "lint clean" "$temporary/lint-clean.time"
expect_status "lint clean test" 0 "$code"
expect_empty "lint clean test stdout" "$temporary/lint-clean.stdout"
expect_empty "lint clean test stderr" "$temporary/lint-clean.stderr"

run_timed "$temporary/analyze-findings.stdout" "$temporary/analyze-findings.stderr" \
	"$temporary/analyze-findings.time" \
	"$strider" analyze --only SA1000 "$analyze_dir/findings.go"
code=$?
record_timing "analyze SA1000 true-positive" "$temporary/analyze-findings.time"
expect_status "analyze SA1000 true-positive test" 1 "$code"
expect_file "analyze SA1000 true-positive test" \
	"$analyze_dir/findings.expected" "$temporary/analyze-findings.stdout"
expect_empty "analyze SA1000 true-positive test stderr" "$temporary/analyze-findings.stderr"

run_timed "$temporary/analyze-clean.stdout" "$temporary/analyze-clean.stderr" \
	"$temporary/analyze-clean.time" \
	"$strider" analyze --only SA1000 "$analyze_dir/clean.go"
code=$?
record_timing "analyze SA1000 clean" "$temporary/analyze-clean.time"
expect_status "analyze SA1000 clean test" 0 "$code"
expect_empty "analyze SA1000 clean test stdout" "$temporary/analyze-clean.stdout"
expect_empty "analyze SA1000 clean test stderr" "$temporary/analyze-clean.stderr"

run_timed "$temporary/analyze-sa1001.stdout" "$temporary/analyze-sa1001.stderr" \
	"$temporary/analyze-sa1001.time" \
	"$strider" analyze --only SA1001 "$analyze_dir/sa1001.go"
code=$?
record_timing "analyze SA1001 true-positive" "$temporary/analyze-sa1001.time"
expect_status "analyze SA1001 true-positive test" 1 "$code"
expect_file "analyze SA1001 true-positive test" \
	"$analyze_dir/sa1001.expected" "$temporary/analyze-sa1001.stdout"
expect_empty "analyze SA1001 true-positive test stderr" "$temporary/analyze-sa1001.stderr"

run_timed "$temporary/analyze-sa1001-clean.stdout" \
	"$temporary/analyze-sa1001-clean.stderr" "$temporary/analyze-sa1001-clean.time" \
	"$strider" analyze --only SA1001 "$analyze_dir/sa1001_clean.go"
code=$?
record_timing "analyze SA1001 clean" "$temporary/analyze-sa1001-clean.time"
expect_status "analyze SA1001 clean test" 0 "$code"
expect_empty "analyze SA1001 clean test stdout" "$temporary/analyze-sa1001-clean.stdout"
expect_empty "analyze SA1001 clean test stderr" "$temporary/analyze-sa1001-clean.stderr"

printf 'Timing: curated total Strider time: %ss\n' "$total_seconds"
printf 'curated\t-\ttotal\t%s\t\tINFO\n' "$total_seconds" >> "$timings_file"
if test -n "${GITHUB_STEP_SUMMARY:-}"; then
	printf '| **Total Strider time** | **%ss** |  | INFO |\n' \
		"$total_seconds" >> "$GITHUB_STEP_SUMMARY"
fi
if test "$failed" -ne 0; then
	exit 1
fi
echo "Curated formatter, linter, and analyzer tests passed."
