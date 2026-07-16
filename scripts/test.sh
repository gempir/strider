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
	"$strider" analyze --only invalid-regexp "$analyze_dir/invalid_regexp.go"
code=$?
record_timing "analyze invalid-regexp true-positive" "$temporary/analyze-findings.time"
expect_status "analyze invalid-regexp true-positive test" 1 "$code"
expect_file "analyze invalid-regexp true-positive test" \
	"$analyze_dir/invalid_regexp.expected" "$temporary/analyze-findings.stdout"
expect_empty "analyze invalid-regexp true-positive test stderr" "$temporary/analyze-findings.stderr"

run_timed "$temporary/analyze-clean.stdout" "$temporary/analyze-clean.stderr" \
	"$temporary/analyze-clean.time" \
	"$strider" analyze --only invalid-regexp "$analyze_dir/invalid_regexp_clean.go"
code=$?
record_timing "analyze invalid-regexp clean" "$temporary/analyze-clean.time"
expect_status "analyze invalid-regexp clean test" 0 "$code"
expect_empty "analyze invalid-regexp clean test stdout" "$temporary/analyze-clean.stdout"
expect_empty "analyze invalid-regexp clean test stderr" "$temporary/analyze-clean.stderr"

run_timed "$temporary/analyze-invalid_template.stdout" "$temporary/analyze-invalid_template.stderr" \
	"$temporary/analyze-invalid_template.time" \
	"$strider" analyze --only invalid-template "$analyze_dir/invalid_template.go"
code=$?
record_timing "analyze invalid-template true-positive" "$temporary/analyze-invalid_template.time"
expect_status "analyze invalid-template true-positive test" 1 "$code"
expect_file "analyze invalid-template true-positive test" \
	"$analyze_dir/invalid_template.expected" "$temporary/analyze-invalid_template.stdout"
expect_empty "analyze invalid-template true-positive test stderr" "$temporary/analyze-invalid_template.stderr"

run_timed "$temporary/analyze-invalid_template-clean.stdout" \
	"$temporary/analyze-invalid_template-clean.stderr" "$temporary/analyze-invalid_template-clean.time" \
	"$strider" analyze --only invalid-template "$analyze_dir/invalid_template_clean.go"
code=$?
record_timing "analyze invalid-template clean" "$temporary/analyze-invalid_template-clean.time"
expect_status "analyze invalid-template clean test" 0 "$code"
expect_empty "analyze invalid-template clean test stdout" "$temporary/analyze-invalid_template-clean.stdout"
expect_empty "analyze invalid-template clean test stderr" "$temporary/analyze-invalid_template-clean.stderr"

run_timed "$temporary/analyze-invalid_time_layout.stdout" "$temporary/analyze-invalid_time_layout.stderr" \
	"$temporary/analyze-invalid_time_layout.time" \
	"$strider" analyze --only invalid-time-layout "$analyze_dir/invalid_time_layout.go"
code=$?
record_timing "analyze invalid-time-layout true-positive" "$temporary/analyze-invalid_time_layout.time"
expect_status "analyze invalid-time-layout true-positive test" 1 "$code"
expect_file "analyze invalid-time-layout true-positive test" \
	"$analyze_dir/invalid_time_layout.expected" "$temporary/analyze-invalid_time_layout.stdout"
expect_empty "analyze invalid-time-layout true-positive test stderr" "$temporary/analyze-invalid_time_layout.stderr"

run_timed "$temporary/analyze-invalid_time_layout-clean.stdout" \
	"$temporary/analyze-invalid_time_layout-clean.stderr" "$temporary/analyze-invalid_time_layout-clean.time" \
	"$strider" analyze --only invalid-time-layout "$analyze_dir/invalid_time_layout_clean.go"
code=$?
record_timing "analyze invalid-time-layout clean" "$temporary/analyze-invalid_time_layout-clean.time"
expect_status "analyze invalid-time-layout clean test" 0 "$code"
expect_empty "analyze invalid-time-layout clean test stdout" "$temporary/analyze-invalid_time_layout-clean.stdout"
expect_empty "analyze invalid-time-layout clean test stderr" "$temporary/analyze-invalid_time_layout-clean.stderr"

run_timed "$temporary/analyze-unsupported_binary_write.stdout" "$temporary/analyze-unsupported_binary_write.stderr" \
	"$temporary/analyze-unsupported_binary_write.time" \
	"$strider" analyze --only unsupported-binary-write "$analyze_dir/unsupported_binary_write.go"
code=$?
record_timing "analyze unsupported-binary-write true-positive" "$temporary/analyze-unsupported_binary_write.time"
expect_status "analyze unsupported-binary-write true-positive test" 1 "$code"
expect_file "analyze unsupported-binary-write true-positive test" \
	"$analyze_dir/unsupported_binary_write.expected" "$temporary/analyze-unsupported_binary_write.stdout"
expect_empty "analyze unsupported-binary-write true-positive test stderr" "$temporary/analyze-unsupported_binary_write.stderr"

run_timed "$temporary/analyze-unsupported_binary_write-clean.stdout" \
	"$temporary/analyze-unsupported_binary_write-clean.stderr" "$temporary/analyze-unsupported_binary_write-clean.time" \
	"$strider" analyze --only unsupported-binary-write "$analyze_dir/unsupported_binary_write_clean.go"
code=$?
record_timing "analyze unsupported-binary-write clean" "$temporary/analyze-unsupported_binary_write-clean.time"
expect_status "analyze unsupported-binary-write clean test" 0 "$code"
expect_empty "analyze unsupported-binary-write clean test stdout" "$temporary/analyze-unsupported_binary_write-clean.stdout"
expect_empty "analyze unsupported-binary-write clean test stderr" "$temporary/analyze-unsupported_binary_write-clean.stderr"

run_timed "$temporary/analyze-suspicious_sleep.stdout" "$temporary/analyze-suspicious_sleep.stderr" \
	"$temporary/analyze-suspicious_sleep.time" \
	"$strider" analyze --only suspicious-sleep "$analyze_dir/suspicious_sleep.go"
code=$?
record_timing "analyze suspicious-sleep true-positive" "$temporary/analyze-suspicious_sleep.time"
expect_status "analyze suspicious-sleep true-positive test" 1 "$code"
expect_file "analyze suspicious-sleep true-positive test" \
	"$analyze_dir/suspicious_sleep.expected" "$temporary/analyze-suspicious_sleep.stdout"
expect_empty "analyze suspicious-sleep true-positive test stderr" "$temporary/analyze-suspicious_sleep.stderr"

run_timed "$temporary/analyze-suspicious_sleep-clean.stdout" \
	"$temporary/analyze-suspicious_sleep-clean.stderr" "$temporary/analyze-suspicious_sleep-clean.time" \
	"$strider" analyze --only suspicious-sleep "$analyze_dir/suspicious_sleep_clean.go"
code=$?
record_timing "analyze suspicious-sleep clean" "$temporary/analyze-suspicious_sleep-clean.time"
expect_status "analyze suspicious-sleep clean test" 0 "$code"
expect_empty "analyze suspicious-sleep clean test stdout" "$temporary/analyze-suspicious_sleep-clean.stdout"
expect_empty "analyze suspicious-sleep clean test stderr" "$temporary/analyze-suspicious_sleep-clean.stderr"

run_timed "$temporary/analyze-invalid_exec_command.stdout" "$temporary/analyze-invalid_exec_command.stderr" \
	"$temporary/analyze-invalid_exec_command.time" \
	"$strider" analyze --only invalid-exec-command "$analyze_dir/invalid_exec_command.go"
code=$?
record_timing "analyze invalid-exec-command true-positive" "$temporary/analyze-invalid_exec_command.time"
expect_status "analyze invalid-exec-command true-positive test" 1 "$code"
expect_file "analyze invalid-exec-command true-positive test" \
	"$analyze_dir/invalid_exec_command.expected" "$temporary/analyze-invalid_exec_command.stdout"
expect_empty "analyze invalid-exec-command true-positive test stderr" "$temporary/analyze-invalid_exec_command.stderr"

run_timed "$temporary/analyze-invalid_exec_command-clean.stdout" \
	"$temporary/analyze-invalid_exec_command-clean.stderr" "$temporary/analyze-invalid_exec_command-clean.time" \
	"$strider" analyze --only invalid-exec-command "$analyze_dir/invalid_exec_command_clean.go"
code=$?
record_timing "analyze invalid-exec-command clean" "$temporary/analyze-invalid_exec_command-clean.time"
expect_status "analyze invalid-exec-command clean test" 0 "$code"
expect_empty "analyze invalid-exec-command clean test stdout" "$temporary/analyze-invalid_exec_command-clean.stdout"
expect_empty "analyze invalid-exec-command clean test stderr" "$temporary/analyze-invalid_exec_command-clean.stderr"

run_timed "$temporary/analyze-dynamic_printf.stdout" "$temporary/analyze-dynamic_printf.stderr" \
	"$temporary/analyze-dynamic_printf.time" \
	"$strider" analyze --only dynamic-printf "$analyze_dir/dynamic_printf.go"
code=$?
record_timing "analyze dynamic-printf true-positive" "$temporary/analyze-dynamic_printf.time"
expect_status "analyze dynamic-printf true-positive test" 1 "$code"
expect_file "analyze dynamic-printf true-positive test" \
	"$analyze_dir/dynamic_printf.expected" "$temporary/analyze-dynamic_printf.stdout"
expect_empty "analyze dynamic-printf true-positive test stderr" "$temporary/analyze-dynamic_printf.stderr"

run_timed "$temporary/analyze-dynamic_printf-clean.stdout" \
	"$temporary/analyze-dynamic_printf-clean.stderr" "$temporary/analyze-dynamic_printf-clean.time" \
	"$strider" analyze --only dynamic-printf "$analyze_dir/dynamic_printf_clean.go"
code=$?
record_timing "analyze dynamic-printf clean" "$temporary/analyze-dynamic_printf-clean.time"
expect_status "analyze dynamic-printf clean test" 0 "$code"
expect_empty "analyze dynamic-printf clean test stdout" "$temporary/analyze-dynamic_printf-clean.stdout"
expect_empty "analyze dynamic-printf clean test stderr" "$temporary/analyze-dynamic_printf-clean.stderr"

run_timed "$temporary/analyze-invalid_url.stdout" "$temporary/analyze-invalid_url.stderr" \
	"$temporary/analyze-invalid_url.time" \
	"$strider" analyze --only invalid-url "$analyze_dir/invalid_url.go"
code=$?
record_timing "analyze invalid-url true-positive" "$temporary/analyze-invalid_url.time"
expect_status "analyze invalid-url true-positive test" 1 "$code"
expect_file "analyze invalid-url true-positive test" \
	"$analyze_dir/invalid_url.expected" "$temporary/analyze-invalid_url.stdout"
expect_empty "analyze invalid-url true-positive test stderr" "$temporary/analyze-invalid_url.stderr"

run_timed "$temporary/analyze-invalid_url-clean.stdout" \
	"$temporary/analyze-invalid_url-clean.stderr" "$temporary/analyze-invalid_url-clean.time" \
	"$strider" analyze --only invalid-url "$analyze_dir/invalid_url_clean.go"
code=$?
record_timing "analyze invalid-url clean" "$temporary/analyze-invalid_url-clean.time"
expect_status "analyze invalid-url clean test" 0 "$code"
expect_empty "analyze invalid-url clean test stdout" "$temporary/analyze-invalid_url-clean.stdout"
expect_empty "analyze invalid-url clean test stderr" "$temporary/analyze-invalid_url-clean.stderr"

run_timed "$temporary/analyze-non_canonical_header.stdout" "$temporary/analyze-non_canonical_header.stderr" \
	"$temporary/analyze-non_canonical_header.time" \
	"$strider" analyze --only non-canonical-header "$analyze_dir/non_canonical_header.go"
code=$?
record_timing "analyze non-canonical-header true-positive" "$temporary/analyze-non_canonical_header.time"
expect_status "analyze non-canonical-header true-positive test" 1 "$code"
expect_file "analyze non-canonical-header true-positive test" \
	"$analyze_dir/non_canonical_header.expected" "$temporary/analyze-non_canonical_header.stdout"
expect_empty "analyze non-canonical-header true-positive test stderr" "$temporary/analyze-non_canonical_header.stderr"

run_timed "$temporary/analyze-non_canonical_header-clean.stdout" \
	"$temporary/analyze-non_canonical_header-clean.stderr" "$temporary/analyze-non_canonical_header-clean.time" \
	"$strider" analyze --only non-canonical-header "$analyze_dir/non_canonical_header_clean.go"
code=$?
record_timing "analyze non-canonical-header clean" "$temporary/analyze-non_canonical_header-clean.time"
expect_status "analyze non-canonical-header clean test" 0 "$code"
expect_empty "analyze non-canonical-header clean test stdout" "$temporary/analyze-non_canonical_header-clean.stdout"
expect_empty "analyze non-canonical-header clean test stderr" "$temporary/analyze-non_canonical_header-clean.stderr"

run_timed "$temporary/analyze-regexp_find_all_zero.stdout" "$temporary/analyze-regexp_find_all_zero.stderr" \
	"$temporary/analyze-regexp_find_all_zero.time" \
	"$strider" analyze --only regexp-find-all-zero "$analyze_dir/regexp_find_all_zero.go"
code=$?
record_timing "analyze regexp-find-all-zero true-positive" "$temporary/analyze-regexp_find_all_zero.time"
expect_status "analyze regexp-find-all-zero true-positive test" 1 "$code"
expect_file "analyze regexp-find-all-zero true-positive test" \
	"$analyze_dir/regexp_find_all_zero.expected" "$temporary/analyze-regexp_find_all_zero.stdout"
expect_empty "analyze regexp-find-all-zero true-positive test stderr" "$temporary/analyze-regexp_find_all_zero.stderr"

run_timed "$temporary/analyze-regexp_find_all_zero-clean.stdout" \
	"$temporary/analyze-regexp_find_all_zero-clean.stderr" "$temporary/analyze-regexp_find_all_zero-clean.time" \
	"$strider" analyze --only regexp-find-all-zero "$analyze_dir/regexp_find_all_zero_clean.go"
code=$?
record_timing "analyze regexp-find-all-zero clean" "$temporary/analyze-regexp_find_all_zero-clean.time"
expect_status "analyze regexp-find-all-zero clean test" 0 "$code"
expect_empty "analyze regexp-find-all-zero clean test stdout" "$temporary/analyze-regexp_find_all_zero-clean.stdout"
expect_empty "analyze regexp-find-all-zero clean test stderr" "$temporary/analyze-regexp_find_all_zero-clean.stderr"

run_timed "$temporary/analyze-invalid_utf8.stdout" "$temporary/analyze-invalid_utf8.stderr" \
	"$temporary/analyze-invalid_utf8.time" \
	"$strider" analyze --only invalid-utf8 "$analyze_dir/invalid_utf8.go"
code=$?
record_timing "analyze invalid-utf8 true-positive" "$temporary/analyze-invalid_utf8.time"
expect_status "analyze invalid-utf8 true-positive test" 1 "$code"
expect_file "analyze invalid-utf8 true-positive test" \
	"$analyze_dir/invalid_utf8.expected" "$temporary/analyze-invalid_utf8.stdout"
expect_empty "analyze invalid-utf8 true-positive test stderr" "$temporary/analyze-invalid_utf8.stderr"

run_timed "$temporary/analyze-invalid_utf8-clean.stdout" \
	"$temporary/analyze-invalid_utf8-clean.stderr" "$temporary/analyze-invalid_utf8-clean.time" \
	"$strider" analyze --only invalid-utf8 "$analyze_dir/invalid_utf8_clean.go"
code=$?
record_timing "analyze invalid-utf8 clean" "$temporary/analyze-invalid_utf8-clean.time"
expect_status "analyze invalid-utf8 clean test" 0 "$code"
expect_empty "analyze invalid-utf8 clean test stdout" "$temporary/analyze-invalid_utf8-clean.stdout"
expect_empty "analyze invalid-utf8 clean test stderr" "$temporary/analyze-invalid_utf8-clean.stderr"

run_timed "$temporary/analyze-nil_context.stdout" "$temporary/analyze-nil_context.stderr" \
	"$temporary/analyze-nil_context.time" \
	"$strider" analyze --only nil-context "$analyze_dir/nil_context.go"
code=$?
record_timing "analyze nil-context true-positive" "$temporary/analyze-nil_context.time"
expect_status "analyze nil-context true-positive test" 1 "$code"
expect_file "analyze nil-context true-positive test" \
	"$analyze_dir/nil_context.expected" "$temporary/analyze-nil_context.stdout"
expect_empty "analyze nil-context true-positive test stderr" "$temporary/analyze-nil_context.stderr"

run_timed "$temporary/analyze-nil_context-clean.stdout" \
	"$temporary/analyze-nil_context-clean.stderr" "$temporary/analyze-nil_context-clean.time" \
	"$strider" analyze --only nil-context "$analyze_dir/nil_context_clean.go"
code=$?
record_timing "analyze nil-context clean" "$temporary/analyze-nil_context-clean.time"
expect_status "analyze nil-context clean test" 0 "$code"
expect_empty "analyze nil-context clean test stdout" "$temporary/analyze-nil_context-clean.stdout"
expect_empty "analyze nil-context clean test stderr" "$temporary/analyze-nil_context-clean.stderr"

run_timed "$temporary/analyze-swapped_seek_arguments.stdout" "$temporary/analyze-swapped_seek_arguments.stderr" \
	"$temporary/analyze-swapped_seek_arguments.time" \
	"$strider" analyze --only swapped-seek-arguments "$analyze_dir/swapped_seek_arguments.go"
code=$?
record_timing "analyze swapped-seek-arguments true-positive" "$temporary/analyze-swapped_seek_arguments.time"
expect_status "analyze swapped-seek-arguments true-positive test" 1 "$code"
expect_file "analyze swapped-seek-arguments true-positive test" \
	"$analyze_dir/swapped_seek_arguments.expected" "$temporary/analyze-swapped_seek_arguments.stdout"
expect_empty "analyze swapped-seek-arguments true-positive test stderr" "$temporary/analyze-swapped_seek_arguments.stderr"

run_timed "$temporary/analyze-swapped_seek_arguments-clean.stdout" \
	"$temporary/analyze-swapped_seek_arguments-clean.stderr" "$temporary/analyze-swapped_seek_arguments-clean.time" \
	"$strider" analyze --only swapped-seek-arguments "$analyze_dir/swapped_seek_arguments_clean.go"
code=$?
record_timing "analyze swapped-seek-arguments clean" "$temporary/analyze-swapped_seek_arguments-clean.time"
expect_status "analyze swapped-seek-arguments clean test" 0 "$code"
expect_empty "analyze swapped-seek-arguments clean test stdout" "$temporary/analyze-swapped_seek_arguments-clean.stdout"
expect_empty "analyze swapped-seek-arguments clean test stderr" "$temporary/analyze-swapped_seek_arguments-clean.stderr"

run_timed "$temporary/analyze-non_pointer_unmarshal.stdout" \
	"$temporary/analyze-non_pointer_unmarshal.stderr" "$temporary/analyze-non_pointer_unmarshal.time" \
	"$strider" analyze --only non-pointer-unmarshal "$analyze_dir/non_pointer_unmarshal.go"
code=$?
record_timing "analyze non-pointer-unmarshal true-positive" \
	"$temporary/analyze-non_pointer_unmarshal.time"
expect_status "analyze non-pointer-unmarshal true-positive test" 1 "$code"
expect_file "analyze non-pointer-unmarshal true-positive test" \
	"$analyze_dir/non_pointer_unmarshal.expected" "$temporary/analyze-non_pointer_unmarshal.stdout"
expect_empty "analyze non-pointer-unmarshal true-positive test stderr" \
	"$temporary/analyze-non_pointer_unmarshal.stderr"

run_timed "$temporary/analyze-non_pointer_unmarshal-clean.stdout" \
	"$temporary/analyze-non_pointer_unmarshal-clean.stderr" \
	"$temporary/analyze-non_pointer_unmarshal-clean.time" \
	"$strider" analyze --only non-pointer-unmarshal "$analyze_dir/non_pointer_unmarshal_clean.go"
code=$?
record_timing "analyze non-pointer-unmarshal clean" \
	"$temporary/analyze-non_pointer_unmarshal-clean.time"
expect_status "analyze non-pointer-unmarshal clean test" 0 "$code"
expect_empty "analyze non-pointer-unmarshal clean test stdout" \
	"$temporary/analyze-non_pointer_unmarshal-clean.stdout"
expect_empty "analyze non-pointer-unmarshal clean test stderr" \
	"$temporary/analyze-non_pointer_unmarshal-clean.stderr"

run_timed "$temporary/analyze-leaky-time-tick.stdout" \
	"$temporary/analyze-leaky-time-tick.stderr" "$temporary/analyze-leaky-time-tick.time" \
	sh -c 'cd "$1" && exec "$2" analyze --only leaky-time-tick .' sh \
	"$analyze_dir/leaky_time_tick" "$strider"
code=$?
record_timing "analyze leaky-time-tick true-positive" \
	"$temporary/analyze-leaky-time-tick.time"
expect_status "analyze leaky-time-tick true-positive test" 1 "$code"
expect_file "analyze leaky-time-tick true-positive test" \
	"$analyze_dir/leaky_time_tick/expected.txt" "$temporary/analyze-leaky-time-tick.stdout"
expect_empty "analyze leaky-time-tick true-positive test stderr" \
	"$temporary/analyze-leaky-time-tick.stderr"

run_timed "$temporary/analyze-leaky-time-tick-clean.stdout" \
	"$temporary/analyze-leaky-time-tick-clean.stderr" \
	"$temporary/analyze-leaky-time-tick-clean.time" \
	sh -c 'cd "$1" && exec "$2" analyze --only leaky-time-tick .' sh \
	"$analyze_dir/leaky_time_tick_clean" "$strider"
code=$?
record_timing "analyze leaky-time-tick clean" \
	"$temporary/analyze-leaky-time-tick-clean.time"
expect_status "analyze leaky-time-tick clean test" 0 "$code"
expect_empty "analyze leaky-time-tick clean test stdout" \
	"$temporary/analyze-leaky-time-tick-clean.stdout"
expect_empty "analyze leaky-time-tick clean test stderr" \
	"$temporary/analyze-leaky-time-tick-clean.stderr"

run_timed "$temporary/analyze-untrappable-signal.stdout" \
	"$temporary/analyze-untrappable-signal.stderr" \
	"$temporary/analyze-untrappable-signal.time" \
	"$strider" analyze --only untrappable-signal "$analyze_dir/untrappable_signal.go"
code=$?
record_timing "analyze untrappable-signal true-positive" \
	"$temporary/analyze-untrappable-signal.time"
expect_status "analyze untrappable-signal true-positive test" 1 "$code"
expect_file "analyze untrappable-signal true-positive test" \
	"$analyze_dir/untrappable_signal.expected" "$temporary/analyze-untrappable-signal.stdout"
expect_empty "analyze untrappable-signal true-positive test stderr" \
	"$temporary/analyze-untrappable-signal.stderr"

run_timed "$temporary/analyze-untrappable-signal-clean.stdout" \
	"$temporary/analyze-untrappable-signal-clean.stderr" \
	"$temporary/analyze-untrappable-signal-clean.time" \
	"$strider" analyze --only untrappable-signal "$analyze_dir/untrappable_signal_clean.go"
code=$?
record_timing "analyze untrappable-signal clean" \
	"$temporary/analyze-untrappable-signal-clean.time"
expect_status "analyze untrappable-signal clean test" 0 "$code"
expect_empty "analyze untrappable-signal clean test stdout" \
	"$temporary/analyze-untrappable-signal-clean.stdout"
expect_empty "analyze untrappable-signal clean test stderr" \
	"$temporary/analyze-untrappable-signal-clean.stderr"

run_timed "$temporary/analyze-unbuffered-signal-channel.stdout" \
	"$temporary/analyze-unbuffered-signal-channel.stderr" \
	"$temporary/analyze-unbuffered-signal-channel.time" \
	"$strider" analyze --only unbuffered-signal-channel \
	"$analyze_dir/unbuffered_signal_channel.go"
code=$?
record_timing "analyze unbuffered-signal-channel true-positive" \
	"$temporary/analyze-unbuffered-signal-channel.time"
expect_status "analyze unbuffered-signal-channel true-positive test" 1 "$code"
expect_file "analyze unbuffered-signal-channel true-positive test" \
	"$analyze_dir/unbuffered_signal_channel.expected" \
	"$temporary/analyze-unbuffered-signal-channel.stdout"
expect_empty "analyze unbuffered-signal-channel true-positive test stderr" \
	"$temporary/analyze-unbuffered-signal-channel.stderr"

run_timed "$temporary/analyze-unbuffered-signal-channel-clean.stdout" \
	"$temporary/analyze-unbuffered-signal-channel-clean.stderr" \
	"$temporary/analyze-unbuffered-signal-channel-clean.time" \
	"$strider" analyze --only unbuffered-signal-channel \
	"$analyze_dir/unbuffered_signal_channel_clean.go"
code=$?
record_timing "analyze unbuffered-signal-channel clean" \
	"$temporary/analyze-unbuffered-signal-channel-clean.time"
expect_status "analyze unbuffered-signal-channel clean test" 0 "$code"
expect_empty "analyze unbuffered-signal-channel clean test stdout" \
	"$temporary/analyze-unbuffered-signal-channel-clean.stdout"
expect_empty "analyze unbuffered-signal-channel clean test stderr" \
	"$temporary/analyze-unbuffered-signal-channel-clean.stderr"

run_timed "$temporary/analyze-zero-replacement-limit.stdout" \
	"$temporary/analyze-zero-replacement-limit.stderr" \
	"$temporary/analyze-zero-replacement-limit.time" \
	"$strider" analyze --only zero-replacement-limit \
	"$analyze_dir/zero_replacement_limit.go"
code=$?
record_timing "analyze zero-replacement-limit true-positive" \
	"$temporary/analyze-zero-replacement-limit.time"
expect_status "analyze zero-replacement-limit true-positive test" 1 "$code"
expect_file "analyze zero-replacement-limit true-positive test" \
	"$analyze_dir/zero_replacement_limit.expected" \
	"$temporary/analyze-zero-replacement-limit.stdout"
expect_empty "analyze zero-replacement-limit true-positive test stderr" \
	"$temporary/analyze-zero-replacement-limit.stderr"

run_timed "$temporary/analyze-zero-replacement-limit-clean.stdout" \
	"$temporary/analyze-zero-replacement-limit-clean.stderr" \
	"$temporary/analyze-zero-replacement-limit-clean.time" \
	"$strider" analyze --only zero-replacement-limit \
	"$analyze_dir/zero_replacement_limit_clean.go"
code=$?
record_timing "analyze zero-replacement-limit clean" \
	"$temporary/analyze-zero-replacement-limit-clean.time"
expect_status "analyze zero-replacement-limit clean test" 0 "$code"
expect_empty "analyze zero-replacement-limit clean test stdout" \
	"$temporary/analyze-zero-replacement-limit-clean.stdout"
expect_empty "analyze zero-replacement-limit clean test stderr" \
	"$temporary/analyze-zero-replacement-limit-clean.stderr"

run_timed "$temporary/analyze-deprecated-api-usage.stdout" \
	"$temporary/analyze-deprecated-api-usage.stderr" \
	"$temporary/analyze-deprecated-api-usage.time" \
	"$strider" analyze --only deprecated-api-usage \
	"$analyze_dir/deprecated_api_usage.go"
code=$?
record_timing "analyze deprecated-api-usage true-positive" \
	"$temporary/analyze-deprecated-api-usage.time"
expect_status "analyze deprecated-api-usage true-positive test" 1 "$code"
expect_file "analyze deprecated-api-usage true-positive test" \
	"$analyze_dir/deprecated_api_usage.expected" \
	"$temporary/analyze-deprecated-api-usage.stdout"
expect_empty "analyze deprecated-api-usage true-positive test stderr" \
	"$temporary/analyze-deprecated-api-usage.stderr"

run_timed "$temporary/analyze-deprecated-api-usage-clean.stdout" \
	"$temporary/analyze-deprecated-api-usage-clean.stderr" \
	"$temporary/analyze-deprecated-api-usage-clean.time" \
	"$strider" analyze --only deprecated-api-usage \
	"$analyze_dir/deprecated_api_usage_clean.go"
code=$?
record_timing "analyze deprecated-api-usage clean" \
	"$temporary/analyze-deprecated-api-usage-clean.time"
expect_status "analyze deprecated-api-usage clean test" 0 "$code"
expect_empty "analyze deprecated-api-usage clean test stdout" \
	"$temporary/analyze-deprecated-api-usage-clean.stdout"
expect_empty "analyze deprecated-api-usage clean test stderr" \
	"$temporary/analyze-deprecated-api-usage-clean.stderr"

run_timed "$temporary/analyze-invalid-listen-address.stdout" \
	"$temporary/analyze-invalid-listen-address.stderr" \
	"$temporary/analyze-invalid-listen-address.time" \
	"$strider" analyze --only invalid-listen-address \
	"$analyze_dir/invalid_listen_address.go"
code=$?
record_timing "analyze invalid-listen-address true-positive" \
	"$temporary/analyze-invalid-listen-address.time"
expect_status "analyze invalid-listen-address true-positive test" 1 "$code"
expect_file "analyze invalid-listen-address true-positive test" \
	"$analyze_dir/invalid_listen_address.expected" \
	"$temporary/analyze-invalid-listen-address.stdout"
expect_empty "analyze invalid-listen-address true-positive test stderr" \
	"$temporary/analyze-invalid-listen-address.stderr"

run_timed "$temporary/analyze-invalid-listen-address-clean.stdout" \
	"$temporary/analyze-invalid-listen-address-clean.stderr" \
	"$temporary/analyze-invalid-listen-address-clean.time" \
	"$strider" analyze --only invalid-listen-address \
	"$analyze_dir/invalid_listen_address_clean.go"
code=$?
record_timing "analyze invalid-listen-address clean" \
	"$temporary/analyze-invalid-listen-address-clean.time"
expect_status "analyze invalid-listen-address clean test" 0 "$code"
expect_empty "analyze invalid-listen-address clean test stdout" \
	"$temporary/analyze-invalid-listen-address-clean.stdout"
expect_empty "analyze invalid-listen-address clean test stderr" \
	"$temporary/analyze-invalid-listen-address-clean.stderr"

run_timed "$temporary/analyze-ip-byte-comparison.stdout" \
	"$temporary/analyze-ip-byte-comparison.stderr" \
	"$temporary/analyze-ip-byte-comparison.time" \
	"$strider" analyze --only ip-byte-comparison \
	"$analyze_dir/ip_byte_comparison.go"
code=$?
record_timing "analyze ip-byte-comparison true-positive" \
	"$temporary/analyze-ip-byte-comparison.time"
expect_status "analyze ip-byte-comparison true-positive test" 1 "$code"
expect_file "analyze ip-byte-comparison true-positive test" \
	"$analyze_dir/ip_byte_comparison.expected" \
	"$temporary/analyze-ip-byte-comparison.stdout"
expect_empty "analyze ip-byte-comparison true-positive test stderr" \
	"$temporary/analyze-ip-byte-comparison.stderr"

run_timed "$temporary/analyze-ip-byte-comparison-clean.stdout" \
	"$temporary/analyze-ip-byte-comparison-clean.stderr" \
	"$temporary/analyze-ip-byte-comparison-clean.time" \
	"$strider" analyze --only ip-byte-comparison \
	"$analyze_dir/ip_byte_comparison_clean.go"
code=$?
record_timing "analyze ip-byte-comparison clean" \
	"$temporary/analyze-ip-byte-comparison-clean.time"
expect_status "analyze ip-byte-comparison clean test" 0 "$code"
expect_empty "analyze ip-byte-comparison clean test stdout" \
	"$temporary/analyze-ip-byte-comparison-clean.stdout"
expect_empty "analyze ip-byte-comparison clean test stderr" \
	"$temporary/analyze-ip-byte-comparison-clean.stderr"

run_timed "$temporary/analyze-writer-buffer-mutation.stdout" \
	"$temporary/analyze-writer-buffer-mutation.stderr" \
	"$temporary/analyze-writer-buffer-mutation.time" \
	"$strider" analyze --only writer-buffer-mutation \
	"$analyze_dir/writer_buffer_mutation.go"
code=$?
record_timing "analyze writer-buffer-mutation true-positive" \
	"$temporary/analyze-writer-buffer-mutation.time"
expect_status "analyze writer-buffer-mutation true-positive test" 1 "$code"
expect_file "analyze writer-buffer-mutation true-positive test" \
	"$analyze_dir/writer_buffer_mutation.expected" \
	"$temporary/analyze-writer-buffer-mutation.stdout"
expect_empty "analyze writer-buffer-mutation true-positive test stderr" \
	"$temporary/analyze-writer-buffer-mutation.stderr"

run_timed "$temporary/analyze-writer-buffer-mutation-clean.stdout" \
	"$temporary/analyze-writer-buffer-mutation-clean.stderr" \
	"$temporary/analyze-writer-buffer-mutation-clean.time" \
	"$strider" analyze --only writer-buffer-mutation \
	"$analyze_dir/writer_buffer_mutation_clean.go"
code=$?
record_timing "analyze writer-buffer-mutation clean" \
	"$temporary/analyze-writer-buffer-mutation-clean.time"
expect_status "analyze writer-buffer-mutation clean test" 0 "$code"
expect_empty "analyze writer-buffer-mutation clean test stdout" \
	"$temporary/analyze-writer-buffer-mutation-clean.stdout"
expect_empty "analyze writer-buffer-mutation clean test stderr" \
	"$temporary/analyze-writer-buffer-mutation-clean.stderr"

run_timed "$temporary/analyze-duplicate-trim-cutset.stdout" \
	"$temporary/analyze-duplicate-trim-cutset.stderr" \
	"$temporary/analyze-duplicate-trim-cutset.time" \
	"$strider" analyze --only duplicate-trim-cutset \
	"$analyze_dir/duplicate_trim_cutset.go"
code=$?
record_timing "analyze duplicate-trim-cutset true-positive" \
	"$temporary/analyze-duplicate-trim-cutset.time"
expect_status "analyze duplicate-trim-cutset true-positive test" 1 "$code"
expect_file "analyze duplicate-trim-cutset true-positive test" \
	"$analyze_dir/duplicate_trim_cutset.expected" \
	"$temporary/analyze-duplicate-trim-cutset.stdout"
expect_empty "analyze duplicate-trim-cutset true-positive test stderr" \
	"$temporary/analyze-duplicate-trim-cutset.stderr"

run_timed "$temporary/analyze-duplicate-trim-cutset-clean.stdout" \
	"$temporary/analyze-duplicate-trim-cutset-clean.stderr" \
	"$temporary/analyze-duplicate-trim-cutset-clean.time" \
	"$strider" analyze --only duplicate-trim-cutset \
	"$analyze_dir/duplicate_trim_cutset_clean.go"
code=$?
record_timing "analyze duplicate-trim-cutset clean" \
	"$temporary/analyze-duplicate-trim-cutset-clean.time"
expect_status "analyze duplicate-trim-cutset clean test" 0 "$code"
expect_empty "analyze duplicate-trim-cutset clean test stdout" \
	"$temporary/analyze-duplicate-trim-cutset-clean.stdout"
expect_empty "analyze duplicate-trim-cutset clean test stderr" \
	"$temporary/analyze-duplicate-trim-cutset-clean.stderr"

run_timed "$temporary/analyze-timer-reset-drain-race.stdout" \
	"$temporary/analyze-timer-reset-drain-race.stderr" \
	"$temporary/analyze-timer-reset-drain-race.time" \
	"$strider" analyze --only timer-reset-drain-race \
	"$analyze_dir/timer_reset_drain_race.go"
code=$?
record_timing "analyze timer-reset-drain-race true-positive" \
	"$temporary/analyze-timer-reset-drain-race.time"
expect_status "analyze timer-reset-drain-race true-positive test" 1 "$code"
expect_file "analyze timer-reset-drain-race true-positive test" \
	"$analyze_dir/timer_reset_drain_race.expected" \
	"$temporary/analyze-timer-reset-drain-race.stdout"
expect_empty "analyze timer-reset-drain-race true-positive test stderr" \
	"$temporary/analyze-timer-reset-drain-race.stderr"

run_timed "$temporary/analyze-timer-reset-drain-race-clean.stdout" \
	"$temporary/analyze-timer-reset-drain-race-clean.stderr" \
	"$temporary/analyze-timer-reset-drain-race-clean.time" \
	"$strider" analyze --only timer-reset-drain-race \
	"$analyze_dir/timer_reset_drain_race_clean.go"
code=$?
record_timing "analyze timer-reset-drain-race clean" \
	"$temporary/analyze-timer-reset-drain-race-clean.time"
expect_status "analyze timer-reset-drain-race clean test" 0 "$code"
expect_empty "analyze timer-reset-drain-race clean test stdout" \
	"$temporary/analyze-timer-reset-drain-race-clean.stdout"
expect_empty "analyze timer-reset-drain-race clean test stderr" \
	"$temporary/analyze-timer-reset-drain-race-clean.stderr"

run_timed "$temporary/analyze-unsupported-marshal-type.stdout" \
	"$temporary/analyze-unsupported-marshal-type.stderr" \
	"$temporary/analyze-unsupported-marshal-type.time" \
	"$strider" analyze --only unsupported-marshal-type \
	"$analyze_dir/unsupported_marshal_type.go"
code=$?
record_timing "analyze unsupported-marshal-type true-positive" \
	"$temporary/analyze-unsupported-marshal-type.time"
expect_status "analyze unsupported-marshal-type true-positive test" 1 "$code"
expect_file "analyze unsupported-marshal-type true-positive test" \
	"$analyze_dir/unsupported_marshal_type.expected" \
	"$temporary/analyze-unsupported-marshal-type.stdout"
expect_empty "analyze unsupported-marshal-type true-positive test stderr" \
	"$temporary/analyze-unsupported-marshal-type.stderr"

run_timed "$temporary/analyze-unsupported-marshal-type-clean.stdout" \
	"$temporary/analyze-unsupported-marshal-type-clean.stderr" \
	"$temporary/analyze-unsupported-marshal-type-clean.time" \
	"$strider" analyze --only unsupported-marshal-type \
	"$analyze_dir/unsupported_marshal_type_clean.go"
code=$?
record_timing "analyze unsupported-marshal-type clean" \
	"$temporary/analyze-unsupported-marshal-type-clean.time"
expect_status "analyze unsupported-marshal-type clean test" 0 "$code"
expect_empty "analyze unsupported-marshal-type clean test stdout" \
	"$temporary/analyze-unsupported-marshal-type-clean.stdout"
expect_empty "analyze unsupported-marshal-type clean test stderr" \
	"$temporary/analyze-unsupported-marshal-type-clean.stderr"

run_timed "$temporary/analyze-misaligned-atomic-64.stdout" \
	"$temporary/analyze-misaligned-atomic-64.stderr" \
	"$temporary/analyze-misaligned-atomic-64.time" \
	env GOOS=linux GOARCH=386 "$strider" analyze --only misaligned-atomic-64 \
	"$analyze_dir/misaligned_atomic_64.go"
code=$?
record_timing "analyze misaligned-atomic-64 true-positive" \
	"$temporary/analyze-misaligned-atomic-64.time"
expect_status "analyze misaligned-atomic-64 true-positive test" 1 "$code"
expect_file "analyze misaligned-atomic-64 true-positive test" \
	"$analyze_dir/misaligned_atomic_64.expected" \
	"$temporary/analyze-misaligned-atomic-64.stdout"
expect_empty "analyze misaligned-atomic-64 true-positive test stderr" \
	"$temporary/analyze-misaligned-atomic-64.stderr"

run_timed "$temporary/analyze-misaligned-atomic-64-clean.stdout" \
	"$temporary/analyze-misaligned-atomic-64-clean.stderr" \
	"$temporary/analyze-misaligned-atomic-64-clean.time" \
	env GOOS=linux GOARCH=386 "$strider" analyze --only misaligned-atomic-64 \
	"$analyze_dir/misaligned_atomic_64_clean.go"
code=$?
record_timing "analyze misaligned-atomic-64 clean" \
	"$temporary/analyze-misaligned-atomic-64-clean.time"
expect_status "analyze misaligned-atomic-64 clean test" 0 "$code"
expect_empty "analyze misaligned-atomic-64 clean test stdout" \
	"$temporary/analyze-misaligned-atomic-64-clean.stdout"
expect_empty "analyze misaligned-atomic-64 clean test stderr" \
	"$temporary/analyze-misaligned-atomic-64-clean.stderr"

run_timed "$temporary/analyze-sort-non-slice.stdout" \
	"$temporary/analyze-sort-non-slice.stderr" \
	"$temporary/analyze-sort-non-slice.time" \
	"$strider" analyze --only sort-non-slice "$analyze_dir/sort_non_slice.go"
code=$?
record_timing "analyze sort-non-slice true-positive" \
	"$temporary/analyze-sort-non-slice.time"
expect_status "analyze sort-non-slice true-positive test" 1 "$code"
expect_file "analyze sort-non-slice true-positive test" \
	"$analyze_dir/sort_non_slice.expected" "$temporary/analyze-sort-non-slice.stdout"
expect_empty "analyze sort-non-slice true-positive test stderr" \
	"$temporary/analyze-sort-non-slice.stderr"

run_timed "$temporary/analyze-sort-non-slice-clean.stdout" \
	"$temporary/analyze-sort-non-slice-clean.stderr" \
	"$temporary/analyze-sort-non-slice-clean.time" \
	"$strider" analyze --only sort-non-slice "$analyze_dir/sort_non_slice_clean.go"
code=$?
record_timing "analyze sort-non-slice clean" \
	"$temporary/analyze-sort-non-slice-clean.time"
expect_status "analyze sort-non-slice clean test" 0 "$code"
expect_empty "analyze sort-non-slice clean test stdout" \
	"$temporary/analyze-sort-non-slice-clean.stdout"
expect_empty "analyze sort-non-slice clean test stderr" \
	"$temporary/analyze-sort-non-slice-clean.stderr"

run_timed "$temporary/analyze-context-key-type.stdout" \
	"$temporary/analyze-context-key-type.stderr" \
	"$temporary/analyze-context-key-type.time" \
	"$strider" analyze --only context-key-type "$analyze_dir/context_key_type.go"
code=$?
record_timing "analyze context-key-type true-positive" \
	"$temporary/analyze-context-key-type.time"
expect_status "analyze context-key-type true-positive test" 1 "$code"
expect_file "analyze context-key-type true-positive test" \
	"$analyze_dir/context_key_type.expected" "$temporary/analyze-context-key-type.stdout"
expect_empty "analyze context-key-type true-positive test stderr" \
	"$temporary/analyze-context-key-type.stderr"

run_timed "$temporary/analyze-context-key-type-clean.stdout" \
	"$temporary/analyze-context-key-type-clean.stderr" \
	"$temporary/analyze-context-key-type-clean.time" \
	"$strider" analyze --only context-key-type "$analyze_dir/context_key_type_clean.go"
code=$?
record_timing "analyze context-key-type clean" \
	"$temporary/analyze-context-key-type-clean.time"
expect_status "analyze context-key-type clean test" 0 "$code"
expect_empty "analyze context-key-type clean test stdout" \
	"$temporary/analyze-context-key-type-clean.stdout"
expect_empty "analyze context-key-type clean test stderr" \
	"$temporary/analyze-context-key-type-clean.stderr"

run_timed "$temporary/analyze-invalid-strconv-argument.stdout" \
	"$temporary/analyze-invalid-strconv-argument.stderr" \
	"$temporary/analyze-invalid-strconv-argument.time" \
	"$strider" analyze --only invalid-strconv-argument \
	"$analyze_dir/invalid_strconv_argument.go"
code=$?
record_timing "analyze invalid-strconv-argument true-positive" \
	"$temporary/analyze-invalid-strconv-argument.time"
expect_status "analyze invalid-strconv-argument true-positive test" 1 "$code"
expect_file "analyze invalid-strconv-argument true-positive test" \
	"$analyze_dir/invalid_strconv_argument.expected" \
	"$temporary/analyze-invalid-strconv-argument.stdout"
expect_empty "analyze invalid-strconv-argument true-positive test stderr" \
	"$temporary/analyze-invalid-strconv-argument.stderr"

run_timed "$temporary/analyze-invalid-strconv-argument-clean.stdout" \
	"$temporary/analyze-invalid-strconv-argument-clean.stderr" \
	"$temporary/analyze-invalid-strconv-argument-clean.time" \
	"$strider" analyze --only invalid-strconv-argument \
	"$analyze_dir/invalid_strconv_argument_clean.go"
code=$?
record_timing "analyze invalid-strconv-argument clean" \
	"$temporary/analyze-invalid-strconv-argument-clean.time"
expect_status "analyze invalid-strconv-argument clean test" 0 "$code"
expect_empty "analyze invalid-strconv-argument clean test stdout" \
	"$temporary/analyze-invalid-strconv-argument-clean.stdout"
expect_empty "analyze invalid-strconv-argument clean test stderr" \
	"$temporary/analyze-invalid-strconv-argument-clean.stderr"

run_timed "$temporary/analyze-overlapping-encode-buffers.stdout" \
	"$temporary/analyze-overlapping-encode-buffers.stderr" \
	"$temporary/analyze-overlapping-encode-buffers.time" \
	"$strider" analyze --only overlapping-encode-buffers \
	"$analyze_dir/overlapping_encode_buffers.go"
code=$?
record_timing "analyze overlapping-encode-buffers true-positive" \
	"$temporary/analyze-overlapping-encode-buffers.time"
expect_status "analyze overlapping-encode-buffers true-positive test" 1 "$code"
expect_file "analyze overlapping-encode-buffers true-positive test" \
	"$analyze_dir/overlapping_encode_buffers.expected" \
	"$temporary/analyze-overlapping-encode-buffers.stdout"
expect_empty "analyze overlapping-encode-buffers true-positive test stderr" \
	"$temporary/analyze-overlapping-encode-buffers.stderr"

run_timed "$temporary/analyze-overlapping-encode-buffers-clean.stdout" \
	"$temporary/analyze-overlapping-encode-buffers-clean.stderr" \
	"$temporary/analyze-overlapping-encode-buffers-clean.time" \
	"$strider" analyze --only overlapping-encode-buffers \
	"$analyze_dir/overlapping_encode_buffers_clean.go"
code=$?
record_timing "analyze overlapping-encode-buffers clean" \
	"$temporary/analyze-overlapping-encode-buffers-clean.time"
expect_status "analyze overlapping-encode-buffers clean test" 0 "$code"
expect_empty "analyze overlapping-encode-buffers clean test stdout" \
	"$temporary/analyze-overlapping-encode-buffers-clean.stdout"
expect_empty "analyze overlapping-encode-buffers clean test stderr" \
	"$temporary/analyze-overlapping-encode-buffers-clean.stderr"

run_timed "$temporary/analyze-swapped-errors-is-arguments.stdout" \
	"$temporary/analyze-swapped-errors-is-arguments.stderr" \
	"$temporary/analyze-swapped-errors-is-arguments.time" \
	"$strider" analyze --only swapped-errors-is-arguments \
	"$analyze_dir/swapped_errors_is_arguments.go"
code=$?
record_timing "analyze swapped-errors-is-arguments true-positive" \
	"$temporary/analyze-swapped-errors-is-arguments.time"
expect_status "analyze swapped-errors-is-arguments true-positive test" 1 "$code"
expect_file "analyze swapped-errors-is-arguments true-positive test" \
	"$analyze_dir/swapped_errors_is_arguments.expected" \
	"$temporary/analyze-swapped-errors-is-arguments.stdout"
expect_empty "analyze swapped-errors-is-arguments true-positive test stderr" \
	"$temporary/analyze-swapped-errors-is-arguments.stderr"

run_timed "$temporary/analyze-swapped-errors-is-arguments-clean.stdout" \
	"$temporary/analyze-swapped-errors-is-arguments-clean.stderr" \
	"$temporary/analyze-swapped-errors-is-arguments-clean.time" \
	"$strider" analyze --only swapped-errors-is-arguments \
	"$analyze_dir/swapped_errors_is_arguments_clean.go"
code=$?
record_timing "analyze swapped-errors-is-arguments clean" \
	"$temporary/analyze-swapped-errors-is-arguments-clean.time"
expect_status "analyze swapped-errors-is-arguments clean test" 0 "$code"
expect_empty "analyze swapped-errors-is-arguments clean test stdout" \
	"$temporary/analyze-swapped-errors-is-arguments-clean.stdout"
expect_empty "analyze swapped-errors-is-arguments clean test stderr" \
	"$temporary/analyze-swapped-errors-is-arguments-clean.stderr"

run_timed "$temporary/analyze-waitgroup-add-inside-goroutine.stdout" \
	"$temporary/analyze-waitgroup-add-inside-goroutine.stderr" \
	"$temporary/analyze-waitgroup-add-inside-goroutine.time" \
	"$strider" analyze --only waitgroup-add-inside-goroutine \
	"$analyze_dir/waitgroup_add_inside_goroutine.go"
code=$?
record_timing "analyze waitgroup-add-inside-goroutine true-positive" \
	"$temporary/analyze-waitgroup-add-inside-goroutine.time"
expect_status "analyze waitgroup-add-inside-goroutine true-positive test" 1 "$code"
expect_file "analyze waitgroup-add-inside-goroutine true-positive test" \
	"$analyze_dir/waitgroup_add_inside_goroutine.expected" \
	"$temporary/analyze-waitgroup-add-inside-goroutine.stdout"
expect_empty "analyze waitgroup-add-inside-goroutine true-positive test stderr" \
	"$temporary/analyze-waitgroup-add-inside-goroutine.stderr"

run_timed "$temporary/analyze-waitgroup-add-inside-goroutine-clean.stdout" \
	"$temporary/analyze-waitgroup-add-inside-goroutine-clean.stderr" \
	"$temporary/analyze-waitgroup-add-inside-goroutine-clean.time" \
	"$strider" analyze --only waitgroup-add-inside-goroutine \
	"$analyze_dir/waitgroup_add_inside_goroutine_clean.go"
code=$?
record_timing "analyze waitgroup-add-inside-goroutine clean" \
	"$temporary/analyze-waitgroup-add-inside-goroutine-clean.time"
expect_status "analyze waitgroup-add-inside-goroutine clean test" 0 "$code"
expect_empty "analyze waitgroup-add-inside-goroutine clean test stdout" \
	"$temporary/analyze-waitgroup-add-inside-goroutine-clean.stdout"
expect_empty "analyze waitgroup-add-inside-goroutine clean test stderr" \
	"$temporary/analyze-waitgroup-add-inside-goroutine-clean.stderr"

run_timed "$temporary/analyze-empty-critical-section.stdout" \
	"$temporary/analyze-empty-critical-section.stderr" \
	"$temporary/analyze-empty-critical-section.time" \
	"$strider" analyze --only empty-critical-section \
	"$analyze_dir/empty_critical_section.go"
code=$?
record_timing "analyze empty-critical-section true-positive" \
	"$temporary/analyze-empty-critical-section.time"
expect_status "analyze empty-critical-section true-positive test" 1 "$code"
expect_file "analyze empty-critical-section true-positive test" \
	"$analyze_dir/empty_critical_section.expected" \
	"$temporary/analyze-empty-critical-section.stdout"
expect_empty "analyze empty-critical-section true-positive test stderr" \
	"$temporary/analyze-empty-critical-section.stderr"

run_timed "$temporary/analyze-empty-critical-section-clean.stdout" \
	"$temporary/analyze-empty-critical-section-clean.stderr" \
	"$temporary/analyze-empty-critical-section-clean.time" \
	"$strider" analyze --only empty-critical-section \
	"$analyze_dir/empty_critical_section_clean.go"
code=$?
record_timing "analyze empty-critical-section clean" \
	"$temporary/analyze-empty-critical-section-clean.time"
expect_status "analyze empty-critical-section clean test" 0 "$code"
expect_empty "analyze empty-critical-section clean test stdout" \
	"$temporary/analyze-empty-critical-section-clean.stdout"
expect_empty "analyze empty-critical-section clean test stderr" \
	"$temporary/analyze-empty-critical-section-clean.stderr"

run_timed "$temporary/analyze-testing-fatal-in-goroutine.stdout" \
	"$temporary/analyze-testing-fatal-in-goroutine.stderr" \
	"$temporary/analyze-testing-fatal-in-goroutine.time" \
	"$strider" analyze --only testing-fatal-in-goroutine \
	"$analyze_dir/testing_fatal_in_goroutine_test.go"
code=$?
record_timing "analyze testing-fatal-in-goroutine true-positive" \
	"$temporary/analyze-testing-fatal-in-goroutine.time"
expect_status "analyze testing-fatal-in-goroutine true-positive test" 1 "$code"
expect_file "analyze testing-fatal-in-goroutine true-positive test" \
	"$analyze_dir/testing_fatal_in_goroutine.expected" \
	"$temporary/analyze-testing-fatal-in-goroutine.stdout"
expect_empty "analyze testing-fatal-in-goroutine true-positive test stderr" \
	"$temporary/analyze-testing-fatal-in-goroutine.stderr"

run_timed "$temporary/analyze-testing-fatal-in-goroutine-clean.stdout" \
	"$temporary/analyze-testing-fatal-in-goroutine-clean.stderr" \
	"$temporary/analyze-testing-fatal-in-goroutine-clean.time" \
	"$strider" analyze --only testing-fatal-in-goroutine \
	"$analyze_dir/testing_fatal_in_goroutine_clean_test.go"
code=$?
record_timing "analyze testing-fatal-in-goroutine clean" \
	"$temporary/analyze-testing-fatal-in-goroutine-clean.time"
expect_status "analyze testing-fatal-in-goroutine clean test" 0 "$code"
expect_empty "analyze testing-fatal-in-goroutine clean test stdout" \
	"$temporary/analyze-testing-fatal-in-goroutine-clean.stdout"
expect_empty "analyze testing-fatal-in-goroutine clean test stderr" \
	"$temporary/analyze-testing-fatal-in-goroutine-clean.stderr"

run_timed "$temporary/analyze-deferred-lock-after-lock.stdout" \
	"$temporary/analyze-deferred-lock-after-lock.stderr" \
	"$temporary/analyze-deferred-lock-after-lock.time" \
	"$strider" analyze --only deferred-lock-after-lock \
	"$analyze_dir/deferred_lock_after_lock.go"
code=$?
record_timing "analyze deferred-lock-after-lock true-positive" \
	"$temporary/analyze-deferred-lock-after-lock.time"
expect_status "analyze deferred-lock-after-lock true-positive test" 1 "$code"
expect_file "analyze deferred-lock-after-lock true-positive test" \
	"$analyze_dir/deferred_lock_after_lock.expected" \
	"$temporary/analyze-deferred-lock-after-lock.stdout"
expect_empty "analyze deferred-lock-after-lock true-positive test stderr" \
	"$temporary/analyze-deferred-lock-after-lock.stderr"

run_timed "$temporary/analyze-deferred-lock-after-lock-clean.stdout" \
	"$temporary/analyze-deferred-lock-after-lock-clean.stderr" \
	"$temporary/analyze-deferred-lock-after-lock-clean.time" \
	"$strider" analyze --only deferred-lock-after-lock \
	"$analyze_dir/deferred_lock_after_lock_clean.go"
code=$?
record_timing "analyze deferred-lock-after-lock clean" \
	"$temporary/analyze-deferred-lock-after-lock-clean.time"
expect_status "analyze deferred-lock-after-lock clean test" 0 "$code"
expect_empty "analyze deferred-lock-after-lock clean test stdout" \
	"$temporary/analyze-deferred-lock-after-lock-clean.stdout"
expect_empty "analyze deferred-lock-after-lock clean test stderr" \
	"$temporary/analyze-deferred-lock-after-lock-clean.stderr"

run_timed "$temporary/analyze-testmain-missing-exit.stdout" \
	"$temporary/analyze-testmain-missing-exit.stderr" \
	"$temporary/analyze-testmain-missing-exit.time" \
	sh -c 'cd "$1" && exec "$2" analyze --only testmain-missing-exit .' sh \
	"$analyze_dir/testmain_missing_exit" "$strider"
code=$?
record_timing "analyze testmain-missing-exit true-positive" \
	"$temporary/analyze-testmain-missing-exit.time"
expect_status "analyze testmain-missing-exit true-positive test" 1 "$code"
expect_file "analyze testmain-missing-exit true-positive test" \
	"$analyze_dir/testmain_missing_exit/expected.txt" \
	"$temporary/analyze-testmain-missing-exit.stdout"
expect_empty "analyze testmain-missing-exit true-positive test stderr" \
	"$temporary/analyze-testmain-missing-exit.stderr"

run_timed "$temporary/analyze-testmain-missing-exit-clean.stdout" \
	"$temporary/analyze-testmain-missing-exit-clean.stderr" \
	"$temporary/analyze-testmain-missing-exit-clean.time" \
	sh -c 'cd "$1" && exec "$2" analyze --only testmain-missing-exit .' sh \
	"$analyze_dir/testmain_missing_exit_clean" "$strider"
code=$?
record_timing "analyze testmain-missing-exit clean" \
	"$temporary/analyze-testmain-missing-exit-clean.time"
expect_status "analyze testmain-missing-exit clean test" 0 "$code"
expect_empty "analyze testmain-missing-exit clean test stdout" \
	"$temporary/analyze-testmain-missing-exit-clean.stdout"
expect_empty "analyze testmain-missing-exit clean test stderr" \
	"$temporary/analyze-testmain-missing-exit-clean.stderr"

run_timed "$temporary/analyze-benchmark-iteration-mutation.stdout" \
	"$temporary/analyze-benchmark-iteration-mutation.stderr" \
	"$temporary/analyze-benchmark-iteration-mutation.time" \
	"$strider" analyze --only benchmark-iteration-mutation \
	"$analyze_dir/benchmark_iteration_mutation_test.go"
code=$?
record_timing "analyze benchmark-iteration-mutation true-positive" \
	"$temporary/analyze-benchmark-iteration-mutation.time"
expect_status "analyze benchmark-iteration-mutation true-positive test" 1 "$code"
expect_file "analyze benchmark-iteration-mutation true-positive test" \
	"$analyze_dir/benchmark_iteration_mutation.expected" \
	"$temporary/analyze-benchmark-iteration-mutation.stdout"
expect_empty "analyze benchmark-iteration-mutation true-positive test stderr" \
	"$temporary/analyze-benchmark-iteration-mutation.stderr"

run_timed "$temporary/analyze-benchmark-iteration-mutation-clean.stdout" \
	"$temporary/analyze-benchmark-iteration-mutation-clean.stderr" \
	"$temporary/analyze-benchmark-iteration-mutation-clean.time" \
	"$strider" analyze --only benchmark-iteration-mutation \
	"$analyze_dir/benchmark_iteration_mutation_clean_test.go"
code=$?
record_timing "analyze benchmark-iteration-mutation clean" \
	"$temporary/analyze-benchmark-iteration-mutation-clean.time"
expect_status "analyze benchmark-iteration-mutation clean test" 0 "$code"
expect_empty "analyze benchmark-iteration-mutation clean test stdout" \
	"$temporary/analyze-benchmark-iteration-mutation-clean.stdout"
expect_empty "analyze benchmark-iteration-mutation clean test stderr" \
	"$temporary/analyze-benchmark-iteration-mutation-clean.stderr"

run_timed "$temporary/analyze-identical-binary-operands.stdout" \
	"$temporary/analyze-identical-binary-operands.stderr" \
	"$temporary/analyze-identical-binary-operands.time" \
	"$strider" analyze --only identical-binary-operands \
	"$analyze_dir/identical_binary_operands.go"
code=$?
record_timing "analyze identical-binary-operands true-positive" \
	"$temporary/analyze-identical-binary-operands.time"
expect_status "analyze identical-binary-operands true-positive test" 1 "$code"
expect_file "analyze identical-binary-operands true-positive test" \
	"$analyze_dir/identical_binary_operands.expected" \
	"$temporary/analyze-identical-binary-operands.stdout"
expect_empty "analyze identical-binary-operands true-positive test stderr" \
	"$temporary/analyze-identical-binary-operands.stderr"

run_timed "$temporary/analyze-identical-binary-operands-clean.stdout" \
	"$temporary/analyze-identical-binary-operands-clean.stderr" \
	"$temporary/analyze-identical-binary-operands-clean.time" \
	"$strider" analyze --only identical-binary-operands \
	"$analyze_dir/identical_binary_operands_clean.go"
code=$?
record_timing "analyze identical-binary-operands clean" \
	"$temporary/analyze-identical-binary-operands-clean.time"
expect_status "analyze identical-binary-operands clean test" 0 "$code"
expect_empty "analyze identical-binary-operands clean test stdout" \
	"$temporary/analyze-identical-binary-operands-clean.stdout"
expect_empty "analyze identical-binary-operands clean test stderr" \
	"$temporary/analyze-identical-binary-operands-clean.stderr"

run_timed "$temporary/analyze-impossible-integer-comparison.stdout" \
	"$temporary/analyze-impossible-integer-comparison.stderr" \
	"$temporary/analyze-impossible-integer-comparison.time" \
	"$strider" analyze --only impossible-integer-comparison \
	"$analyze_dir/impossible_integer_comparison.go"
code=$?
record_timing "analyze impossible-integer-comparison true-positive" \
	"$temporary/analyze-impossible-integer-comparison.time"
expect_status "analyze impossible-integer-comparison true-positive test" 1 "$code"
expect_file "analyze impossible-integer-comparison true-positive test" \
	"$analyze_dir/impossible_integer_comparison.expected" \
	"$temporary/analyze-impossible-integer-comparison.stdout"
expect_empty "analyze impossible-integer-comparison true-positive test stderr" \
	"$temporary/analyze-impossible-integer-comparison.stderr"

run_timed "$temporary/analyze-impossible-integer-comparison-clean.stdout" \
	"$temporary/analyze-impossible-integer-comparison-clean.stderr" \
	"$temporary/analyze-impossible-integer-comparison-clean.time" \
	"$strider" analyze --only impossible-integer-comparison \
	"$analyze_dir/impossible_integer_comparison_clean.go"
code=$?
record_timing "analyze impossible-integer-comparison clean" \
	"$temporary/analyze-impossible-integer-comparison-clean.time"
expect_status "analyze impossible-integer-comparison clean test" 0 "$code"
expect_empty "analyze impossible-integer-comparison clean test stdout" \
	"$temporary/analyze-impossible-integer-comparison-clean.stdout"
expect_empty "analyze impossible-integer-comparison clean test stderr" \
	"$temporary/analyze-impossible-integer-comparison-clean.stderr"

run_timed "$temporary/analyze-single-iteration-loop.stdout" \
	"$temporary/analyze-single-iteration-loop.stderr" \
	"$temporary/analyze-single-iteration-loop.time" \
	"$strider" analyze --only single-iteration-loop \
	"$analyze_dir/single_iteration_loop.go"
code=$?
record_timing "analyze single-iteration-loop true-positive" \
	"$temporary/analyze-single-iteration-loop.time"
expect_status "analyze single-iteration-loop true-positive test" 1 "$code"
expect_file "analyze single-iteration-loop true-positive test" \
	"$analyze_dir/single_iteration_loop.expected" \
	"$temporary/analyze-single-iteration-loop.stdout"
expect_empty "analyze single-iteration-loop true-positive test stderr" \
	"$temporary/analyze-single-iteration-loop.stderr"

run_timed "$temporary/analyze-single-iteration-loop-clean.stdout" \
	"$temporary/analyze-single-iteration-loop-clean.stderr" \
	"$temporary/analyze-single-iteration-loop-clean.time" \
	"$strider" analyze --only single-iteration-loop \
	"$analyze_dir/single_iteration_loop_clean.go"
code=$?
record_timing "analyze single-iteration-loop clean" \
	"$temporary/analyze-single-iteration-loop-clean.time"
expect_status "analyze single-iteration-loop clean test" 0 "$code"
expect_empty "analyze single-iteration-loop clean test stdout" \
	"$temporary/analyze-single-iteration-loop-clean.stdout"
expect_empty "analyze single-iteration-loop clean test stderr" \
	"$temporary/analyze-single-iteration-loop-clean.stderr"

run_timed "$temporary/analyze-ineffective-value-receiver-assignment.stdout" \
	"$temporary/analyze-ineffective-value-receiver-assignment.stderr" \
	"$temporary/analyze-ineffective-value-receiver-assignment.time" \
	"$strider" analyze --only ineffective-value-receiver-assignment \
	"$analyze_dir/ineffective_value_receiver_assignment.go"
code=$?
record_timing "analyze ineffective-value-receiver-assignment true-positive" \
	"$temporary/analyze-ineffective-value-receiver-assignment.time"
expect_status "analyze ineffective-value-receiver-assignment true-positive test" 1 "$code"
expect_file "analyze ineffective-value-receiver-assignment true-positive test" \
	"$analyze_dir/ineffective_value_receiver_assignment.expected" \
	"$temporary/analyze-ineffective-value-receiver-assignment.stdout"
expect_empty "analyze ineffective-value-receiver-assignment true-positive test stderr" \
	"$temporary/analyze-ineffective-value-receiver-assignment.stderr"

run_timed "$temporary/analyze-ineffective-value-receiver-assignment-clean.stdout" \
	"$temporary/analyze-ineffective-value-receiver-assignment-clean.stderr" \
	"$temporary/analyze-ineffective-value-receiver-assignment-clean.time" \
	"$strider" analyze --only ineffective-value-receiver-assignment \
	"$analyze_dir/ineffective_value_receiver_assignment_clean.go"
code=$?
record_timing "analyze ineffective-value-receiver-assignment clean" \
	"$temporary/analyze-ineffective-value-receiver-assignment-clean.time"
expect_status "analyze ineffective-value-receiver-assignment clean test" 0 "$code"
expect_empty "analyze ineffective-value-receiver-assignment clean test stdout" \
	"$temporary/analyze-ineffective-value-receiver-assignment-clean.stdout"
expect_empty "analyze ineffective-value-receiver-assignment clean test stderr" \
	"$temporary/analyze-ineffective-value-receiver-assignment-clean.stderr"

run_timed "$temporary/analyze-overwritten-before-use.stdout" \
	"$temporary/analyze-overwritten-before-use.stderr" \
	"$temporary/analyze-overwritten-before-use.time" \
	"$strider" analyze --only overwritten-before-use \
	"$analyze_dir/overwritten_before_use.go"
code=$?
record_timing "analyze overwritten-before-use true-positive" \
	"$temporary/analyze-overwritten-before-use.time"
expect_status "analyze overwritten-before-use true-positive test" 1 "$code"
expect_file "analyze overwritten-before-use true-positive test" \
	"$analyze_dir/overwritten_before_use.expected" \
	"$temporary/analyze-overwritten-before-use.stdout"
expect_empty "analyze overwritten-before-use true-positive test stderr" \
	"$temporary/analyze-overwritten-before-use.stderr"

run_timed "$temporary/analyze-overwritten-before-use-clean.stdout" \
	"$temporary/analyze-overwritten-before-use-clean.stderr" \
	"$temporary/analyze-overwritten-before-use-clean.time" \
	"$strider" analyze --only overwritten-before-use \
	"$analyze_dir/overwritten_before_use_clean.go"
code=$?
record_timing "analyze overwritten-before-use clean" \
	"$temporary/analyze-overwritten-before-use-clean.time"
expect_status "analyze overwritten-before-use clean test" 0 "$code"
expect_empty "analyze overwritten-before-use clean test stdout" \
	"$temporary/analyze-overwritten-before-use-clean.stdout"
expect_empty "analyze overwritten-before-use clean test stderr" \
	"$temporary/analyze-overwritten-before-use-clean.stderr"

run_timed "$temporary/analyze-unchanged-loop-condition.stdout" \
	"$temporary/analyze-unchanged-loop-condition.stderr" \
	"$temporary/analyze-unchanged-loop-condition.time" \
	"$strider" analyze --only unchanged-loop-condition \
	"$analyze_dir/unchanged_loop_condition.go"
code=$?
record_timing "analyze unchanged-loop-condition true-positive" \
	"$temporary/analyze-unchanged-loop-condition.time"
expect_status "analyze unchanged-loop-condition true-positive test" 1 "$code"
expect_file "analyze unchanged-loop-condition true-positive test" \
	"$analyze_dir/unchanged_loop_condition.expected" \
	"$temporary/analyze-unchanged-loop-condition.stdout"
expect_empty "analyze unchanged-loop-condition true-positive test stderr" \
	"$temporary/analyze-unchanged-loop-condition.stderr"

run_timed "$temporary/analyze-unchanged-loop-condition-clean.stdout" \
	"$temporary/analyze-unchanged-loop-condition-clean.stderr" \
	"$temporary/analyze-unchanged-loop-condition-clean.time" \
	"$strider" analyze --only unchanged-loop-condition \
	"$analyze_dir/unchanged_loop_condition_clean.go"
code=$?
record_timing "analyze unchanged-loop-condition clean" \
	"$temporary/analyze-unchanged-loop-condition-clean.time"
expect_status "analyze unchanged-loop-condition clean test" 0 "$code"
expect_empty "analyze unchanged-loop-condition clean test stdout" \
	"$temporary/analyze-unchanged-loop-condition-clean.stdout"
expect_empty "analyze unchanged-loop-condition clean test stderr" \
	"$temporary/analyze-unchanged-loop-condition-clean.stderr"

run_timed "$temporary/analyze-argument-overwritten-before-use.stdout" \
	"$temporary/analyze-argument-overwritten-before-use.stderr" \
	"$temporary/analyze-argument-overwritten-before-use.time" \
	"$strider" analyze --only argument-overwritten-before-use \
	"$analyze_dir/argument_overwritten_before_use.go"
code=$?
record_timing "analyze argument-overwritten-before-use true-positive" \
	"$temporary/analyze-argument-overwritten-before-use.time"
expect_status "analyze argument-overwritten-before-use true-positive test" 1 "$code"
expect_file "analyze argument-overwritten-before-use true-positive test" \
	"$analyze_dir/argument_overwritten_before_use.expected" \
	"$temporary/analyze-argument-overwritten-before-use.stdout"
expect_empty "analyze argument-overwritten-before-use true-positive test stderr" \
	"$temporary/analyze-argument-overwritten-before-use.stderr"

run_timed "$temporary/analyze-argument-overwritten-before-use-clean.stdout" \
	"$temporary/analyze-argument-overwritten-before-use-clean.stderr" \
	"$temporary/analyze-argument-overwritten-before-use-clean.time" \
	"$strider" analyze --only argument-overwritten-before-use \
	"$analyze_dir/argument_overwritten_before_use_clean.go"
code=$?
record_timing "analyze argument-overwritten-before-use clean" \
	"$temporary/analyze-argument-overwritten-before-use-clean.time"
expect_status "analyze argument-overwritten-before-use clean test" 0 "$code"
expect_empty "analyze argument-overwritten-before-use clean test stdout" \
	"$temporary/analyze-argument-overwritten-before-use-clean.stdout"
expect_empty "analyze argument-overwritten-before-use clean test stderr" \
	"$temporary/analyze-argument-overwritten-before-use-clean.stderr"

run_timed "$temporary/analyze-unused-append-result.stdout" \
	"$temporary/analyze-unused-append-result.stderr" \
	"$temporary/analyze-unused-append-result.time" \
	"$strider" analyze --only unused-append-result \
	"$analyze_dir/unused_append_result.go"
code=$?
record_timing "analyze unused-append-result true-positive" \
	"$temporary/analyze-unused-append-result.time"
expect_status "analyze unused-append-result true-positive test" 1 "$code"
expect_file "analyze unused-append-result true-positive test" \
	"$analyze_dir/unused_append_result.expected" \
	"$temporary/analyze-unused-append-result.stdout"
expect_empty "analyze unused-append-result true-positive test stderr" \
	"$temporary/analyze-unused-append-result.stderr"

run_timed "$temporary/analyze-unused-append-result-clean.stdout" \
	"$temporary/analyze-unused-append-result-clean.stderr" \
	"$temporary/analyze-unused-append-result-clean.time" \
	"$strider" analyze --only unused-append-result \
	"$analyze_dir/unused_append_result_clean.go"
code=$?
record_timing "analyze unused-append-result clean" \
	"$temporary/analyze-unused-append-result-clean.time"
expect_status "analyze unused-append-result clean test" 0 "$code"
expect_empty "analyze unused-append-result clean test stdout" \
	"$temporary/analyze-unused-append-result-clean.stdout"
expect_empty "analyze unused-append-result clean test stderr" \
	"$temporary/analyze-unused-append-result-clean.stderr"

run_timed "$temporary/analyze-nan-comparison.stdout" \
	"$temporary/analyze-nan-comparison.stderr" \
	"$temporary/analyze-nan-comparison.time" \
	"$strider" analyze --only nan-comparison \
	"$analyze_dir/nan_comparison.go"
code=$?
record_timing "analyze nan-comparison true-positive" \
	"$temporary/analyze-nan-comparison.time"
expect_status "analyze nan-comparison true-positive test" 1 "$code"
expect_file "analyze nan-comparison true-positive test" \
	"$analyze_dir/nan_comparison.expected" \
	"$temporary/analyze-nan-comparison.stdout"
expect_empty "analyze nan-comparison true-positive test stderr" \
	"$temporary/analyze-nan-comparison.stderr"

run_timed "$temporary/analyze-nan-comparison-clean.stdout" \
	"$temporary/analyze-nan-comparison-clean.stderr" \
	"$temporary/analyze-nan-comparison-clean.time" \
	"$strider" analyze --only nan-comparison \
	"$analyze_dir/nan_comparison_clean.go"
code=$?
record_timing "analyze nan-comparison clean" \
	"$temporary/analyze-nan-comparison-clean.time"
expect_status "analyze nan-comparison clean test" 0 "$code"
expect_empty "analyze nan-comparison clean test stdout" \
	"$temporary/analyze-nan-comparison-clean.stdout"
expect_empty "analyze nan-comparison clean test stderr" \
	"$temporary/analyze-nan-comparison-clean.stderr"

run_timed "$temporary/analyze-pointless-integer-math.stdout" \
	"$temporary/analyze-pointless-integer-math.stderr" \
	"$temporary/analyze-pointless-integer-math.time" \
	"$strider" analyze --only pointless-integer-math \
	"$analyze_dir/pointless_integer_math.go"
code=$?
record_timing "analyze pointless-integer-math true-positive" \
	"$temporary/analyze-pointless-integer-math.time"
expect_status "analyze pointless-integer-math true-positive test" 1 "$code"
expect_file "analyze pointless-integer-math true-positive test" \
	"$analyze_dir/pointless_integer_math.expected" \
	"$temporary/analyze-pointless-integer-math.stdout"
expect_empty "analyze pointless-integer-math true-positive test stderr" \
	"$temporary/analyze-pointless-integer-math.stderr"

run_timed "$temporary/analyze-pointless-integer-math-clean.stdout" \
	"$temporary/analyze-pointless-integer-math-clean.stderr" \
	"$temporary/analyze-pointless-integer-math-clean.time" \
	"$strider" analyze --only pointless-integer-math \
	"$analyze_dir/pointless_integer_math_clean.go"
code=$?
record_timing "analyze pointless-integer-math clean" \
	"$temporary/analyze-pointless-integer-math-clean.time"
expect_status "analyze pointless-integer-math clean test" 0 "$code"
expect_empty "analyze pointless-integer-math clean test stdout" \
	"$temporary/analyze-pointless-integer-math-clean.stdout"
expect_empty "analyze pointless-integer-math clean test stderr" \
	"$temporary/analyze-pointless-integer-math-clean.stderr"

run_timed "$temporary/analyze-ineffective-bitwise-zero.stdout" \
	"$temporary/analyze-ineffective-bitwise-zero.stderr" \
	"$temporary/analyze-ineffective-bitwise-zero.time" \
	"$strider" analyze --only ineffective-bitwise-zero \
	"$analyze_dir/ineffective_bitwise_zero.go"
code=$?
record_timing "analyze ineffective-bitwise-zero true-positive" \
	"$temporary/analyze-ineffective-bitwise-zero.time"
expect_status "analyze ineffective-bitwise-zero true-positive test" 1 "$code"
expect_file "analyze ineffective-bitwise-zero true-positive test" \
	"$analyze_dir/ineffective_bitwise_zero.expected" \
	"$temporary/analyze-ineffective-bitwise-zero.stdout"
expect_empty "analyze ineffective-bitwise-zero true-positive test stderr" \
	"$temporary/analyze-ineffective-bitwise-zero.stderr"

run_timed "$temporary/analyze-ineffective-bitwise-zero-clean.stdout" \
	"$temporary/analyze-ineffective-bitwise-zero-clean.stderr" \
	"$temporary/analyze-ineffective-bitwise-zero-clean.time" \
	"$strider" analyze --only ineffective-bitwise-zero \
	"$analyze_dir/ineffective_bitwise_zero_clean.go"
code=$?
record_timing "analyze ineffective-bitwise-zero clean" \
	"$temporary/analyze-ineffective-bitwise-zero-clean.time"
expect_status "analyze ineffective-bitwise-zero clean test" 0 "$code"
expect_empty "analyze ineffective-bitwise-zero clean test stdout" \
	"$temporary/analyze-ineffective-bitwise-zero-clean.stdout"
expect_empty "analyze ineffective-bitwise-zero clean test stderr" \
	"$temporary/analyze-ineffective-bitwise-zero-clean.stderr"

run_timed "$temporary/analyze-discarded-pure-result.stdout" \
	"$temporary/analyze-discarded-pure-result.stderr" \
	"$temporary/analyze-discarded-pure-result.time" \
	"$strider" analyze --only discarded-pure-result \
	"$analyze_dir/discarded_pure_result.go"
code=$?
record_timing "analyze discarded-pure-result true-positive" \
	"$temporary/analyze-discarded-pure-result.time"
expect_status "analyze discarded-pure-result true-positive test" 1 "$code"
expect_file "analyze discarded-pure-result true-positive test" \
	"$analyze_dir/discarded_pure_result.expected" \
	"$temporary/analyze-discarded-pure-result.stdout"
expect_empty "analyze discarded-pure-result true-positive test stderr" \
	"$temporary/analyze-discarded-pure-result.stderr"

run_timed "$temporary/analyze-discarded-pure-result-clean.stdout" \
	"$temporary/analyze-discarded-pure-result-clean.stderr" \
	"$temporary/analyze-discarded-pure-result-clean.time" \
	"$strider" analyze --only discarded-pure-result \
	"$analyze_dir/discarded_pure_result_clean.go"
code=$?
record_timing "analyze discarded-pure-result clean" \
	"$temporary/analyze-discarded-pure-result-clean.time"
expect_status "analyze discarded-pure-result clean test" 0 "$code"
expect_empty "analyze discarded-pure-result clean test stdout" \
	"$temporary/analyze-discarded-pure-result-clean.stdout"
expect_empty "analyze discarded-pure-result clean test stderr" \
	"$temporary/analyze-discarded-pure-result-clean.stderr"

run_timed "$temporary/analyze-self-assignment.stdout" \
	"$temporary/analyze-self-assignment.stderr" \
	"$temporary/analyze-self-assignment.time" \
	"$strider" analyze --only self-assignment \
	"$analyze_dir/self_assignment.go"
code=$?
record_timing "analyze self-assignment true-positive" \
	"$temporary/analyze-self-assignment.time"
expect_status "analyze self-assignment true-positive test" 1 "$code"
expect_file "analyze self-assignment true-positive test" \
	"$analyze_dir/self_assignment.expected" \
	"$temporary/analyze-self-assignment.stdout"
expect_empty "analyze self-assignment true-positive test stderr" \
	"$temporary/analyze-self-assignment.stderr"

run_timed "$temporary/analyze-self-assignment-clean.stdout" \
	"$temporary/analyze-self-assignment-clean.stderr" \
	"$temporary/analyze-self-assignment-clean.time" \
	"$strider" analyze --only self-assignment \
	"$analyze_dir/self_assignment_clean.go"
code=$?
record_timing "analyze self-assignment clean" \
	"$temporary/analyze-self-assignment-clean.time"
expect_status "analyze self-assignment clean test" 0 "$code"
expect_empty "analyze self-assignment clean test stdout" \
	"$temporary/analyze-self-assignment-clean.stdout"
expect_empty "analyze self-assignment clean test stderr" \
	"$temporary/analyze-self-assignment-clean.stderr"

run_timed "$temporary/analyze-unreachable-type-switch-case.stdout" \
	"$temporary/analyze-unreachable-type-switch-case.stderr" \
	"$temporary/analyze-unreachable-type-switch-case.time" \
	"$strider" analyze --only unreachable-type-switch-case \
	"$analyze_dir/unreachable_type_switch_case.go"
code=$?
record_timing "analyze unreachable-type-switch-case true-positive" \
	"$temporary/analyze-unreachable-type-switch-case.time"
expect_status "analyze unreachable-type-switch-case true-positive test" 1 "$code"
expect_file "analyze unreachable-type-switch-case true-positive test" \
	"$analyze_dir/unreachable_type_switch_case.expected" \
	"$temporary/analyze-unreachable-type-switch-case.stdout"
expect_empty "analyze unreachable-type-switch-case true-positive test stderr" \
	"$temporary/analyze-unreachable-type-switch-case.stderr"

run_timed "$temporary/analyze-unreachable-type-switch-case-clean.stdout" \
	"$temporary/analyze-unreachable-type-switch-case-clean.stderr" \
	"$temporary/analyze-unreachable-type-switch-case-clean.time" \
	"$strider" analyze --only unreachable-type-switch-case \
	"$analyze_dir/unreachable_type_switch_case_clean.go"
code=$?
record_timing "analyze unreachable-type-switch-case clean" \
	"$temporary/analyze-unreachable-type-switch-case-clean.time"
expect_status "analyze unreachable-type-switch-case clean test" 0 "$code"
expect_empty "analyze unreachable-type-switch-case clean test stdout" \
	"$temporary/analyze-unreachable-type-switch-case-clean.stdout"
expect_empty "analyze unreachable-type-switch-case clean test stderr" \
	"$temporary/analyze-unreachable-type-switch-case-clean.stderr"

run_timed "$temporary/analyze-single-argument-append.stdout" \
	"$temporary/analyze-single-argument-append.stderr" \
	"$temporary/analyze-single-argument-append.time" \
	"$strider" analyze --only single-argument-append \
	"$analyze_dir/single_argument_append.go"
code=$?
record_timing "analyze single-argument-append true-positive" \
	"$temporary/analyze-single-argument-append.time"
expect_status "analyze single-argument-append true-positive test" 1 "$code"
expect_file "analyze single-argument-append true-positive test" \
	"$analyze_dir/single_argument_append.expected" \
	"$temporary/analyze-single-argument-append.stdout"
expect_empty "analyze single-argument-append true-positive test stderr" \
	"$temporary/analyze-single-argument-append.stderr"

run_timed "$temporary/analyze-single-argument-append-clean.stdout" \
	"$temporary/analyze-single-argument-append-clean.stderr" \
	"$temporary/analyze-single-argument-append-clean.time" \
	"$strider" analyze --only single-argument-append \
	"$analyze_dir/single_argument_append_clean.go"
code=$?
record_timing "analyze single-argument-append clean" \
	"$temporary/analyze-single-argument-append-clean.time"
expect_status "analyze single-argument-append clean test" 0 "$code"
expect_empty "analyze single-argument-append clean test stdout" \
	"$temporary/analyze-single-argument-append-clean.stdout"
expect_empty "analyze single-argument-append clean test stderr" \
	"$temporary/analyze-single-argument-append-clean.stderr"

run_timed "$temporary/analyze-address-nil-comparison.stdout" \
	"$temporary/analyze-address-nil-comparison.stderr" \
	"$temporary/analyze-address-nil-comparison.time" \
	"$strider" analyze --only address-nil-comparison \
	"$analyze_dir/address_nil_comparison.go"
code=$?
record_timing "analyze address-nil-comparison true-positive" \
	"$temporary/analyze-address-nil-comparison.time"
expect_status "analyze address-nil-comparison true-positive test" 1 "$code"
expect_file "analyze address-nil-comparison true-positive test" \
	"$analyze_dir/address_nil_comparison.expected" \
	"$temporary/analyze-address-nil-comparison.stdout"
expect_empty "analyze address-nil-comparison true-positive test stderr" \
	"$temporary/analyze-address-nil-comparison.stderr"

run_timed "$temporary/analyze-address-nil-comparison-clean.stdout" \
	"$temporary/analyze-address-nil-comparison-clean.stderr" \
	"$temporary/analyze-address-nil-comparison-clean.time" \
	"$strider" analyze --only address-nil-comparison \
	"$analyze_dir/address_nil_comparison_clean.go"
code=$?
record_timing "analyze address-nil-comparison clean" \
	"$temporary/analyze-address-nil-comparison-clean.time"
expect_status "analyze address-nil-comparison clean test" 0 "$code"
expect_empty "analyze address-nil-comparison clean test stdout" \
	"$temporary/analyze-address-nil-comparison-clean.stdout"
expect_empty "analyze address-nil-comparison clean test stderr" \
	"$temporary/analyze-address-nil-comparison-clean.stderr"

run_timed "$temporary/analyze-impossible-interface-nil-comparison.stdout" \
	"$temporary/analyze-impossible-interface-nil-comparison.stderr" \
	"$temporary/analyze-impossible-interface-nil-comparison.time" \
	"$strider" analyze --only impossible-interface-nil-comparison \
	"$analyze_dir/impossible_interface_nil_comparison.go"
code=$?
record_timing "analyze impossible-interface-nil-comparison true-positive" \
	"$temporary/analyze-impossible-interface-nil-comparison.time"
expect_status "analyze impossible-interface-nil-comparison true-positive test" 1 "$code"
expect_file "analyze impossible-interface-nil-comparison true-positive test" \
	"$analyze_dir/impossible_interface_nil_comparison.expected" \
	"$temporary/analyze-impossible-interface-nil-comparison.stdout"
expect_empty "analyze impossible-interface-nil-comparison true-positive test stderr" \
	"$temporary/analyze-impossible-interface-nil-comparison.stderr"

run_timed "$temporary/analyze-impossible-interface-nil-comparison-clean.stdout" \
	"$temporary/analyze-impossible-interface-nil-comparison-clean.stderr" \
	"$temporary/analyze-impossible-interface-nil-comparison-clean.time" \
	"$strider" analyze --only impossible-interface-nil-comparison \
	"$analyze_dir/impossible_interface_nil_comparison_clean.go"
code=$?
record_timing "analyze impossible-interface-nil-comparison clean" \
	"$temporary/analyze-impossible-interface-nil-comparison-clean.time"
expect_status "analyze impossible-interface-nil-comparison clean test" 0 "$code"
expect_empty "analyze impossible-interface-nil-comparison clean test stdout" \
	"$temporary/analyze-impossible-interface-nil-comparison-clean.stdout"
expect_empty "analyze impossible-interface-nil-comparison clean test stderr" \
	"$temporary/analyze-impossible-interface-nil-comparison-clean.stderr"

run_timed "$temporary/analyze-negative-length-capacity-comparison.stdout" \
	"$temporary/analyze-negative-length-capacity-comparison.stderr" \
	"$temporary/analyze-negative-length-capacity-comparison.time" \
	"$strider" analyze --only negative-length-capacity-comparison \
	"$analyze_dir/negative_length_capacity_comparison.go"
code=$?
record_timing "analyze negative-length-capacity-comparison true-positive" \
	"$temporary/analyze-negative-length-capacity-comparison.time"
expect_status "analyze negative-length-capacity-comparison true-positive test" 1 "$code"
expect_file "analyze negative-length-capacity-comparison true-positive test" \
	"$analyze_dir/negative_length_capacity_comparison.expected" \
	"$temporary/analyze-negative-length-capacity-comparison.stdout"
expect_empty "analyze negative-length-capacity-comparison true-positive test stderr" \
	"$temporary/analyze-negative-length-capacity-comparison.stderr"

run_timed "$temporary/analyze-negative-length-capacity-comparison-clean.stdout" \
	"$temporary/analyze-negative-length-capacity-comparison-clean.stderr" \
	"$temporary/analyze-negative-length-capacity-comparison-clean.time" \
	"$strider" analyze --only negative-length-capacity-comparison \
	"$analyze_dir/negative_length_capacity_comparison_clean.go"
code=$?
record_timing "analyze negative-length-capacity-comparison clean" \
	"$temporary/analyze-negative-length-capacity-comparison-clean.time"
expect_status "analyze negative-length-capacity-comparison clean test" 0 "$code"
expect_empty "analyze negative-length-capacity-comparison clean test stdout" \
	"$temporary/analyze-negative-length-capacity-comparison-clean.stdout"
expect_empty "analyze negative-length-capacity-comparison clean test stderr" \
	"$temporary/analyze-negative-length-capacity-comparison-clean.stderr"

run_timed "$temporary/analyze-constant-negative-zero.stdout" \
	"$temporary/analyze-constant-negative-zero.stderr" \
	"$temporary/analyze-constant-negative-zero.time" \
	"$strider" analyze --only constant-negative-zero \
	"$analyze_dir/constant_negative_zero.go"
code=$?
record_timing "analyze constant-negative-zero true-positive" \
	"$temporary/analyze-constant-negative-zero.time"
expect_status "analyze constant-negative-zero true-positive test" 1 "$code"
expect_file "analyze constant-negative-zero true-positive test" \
	"$analyze_dir/constant_negative_zero.expected" \
	"$temporary/analyze-constant-negative-zero.stdout"
expect_empty "analyze constant-negative-zero true-positive test stderr" \
	"$temporary/analyze-constant-negative-zero.stderr"

run_timed "$temporary/analyze-constant-negative-zero-clean.stdout" \
	"$temporary/analyze-constant-negative-zero-clean.stderr" \
	"$temporary/analyze-constant-negative-zero-clean.time" \
	"$strider" analyze --only constant-negative-zero \
	"$analyze_dir/constant_negative_zero_clean.go"
code=$?
record_timing "analyze constant-negative-zero clean" \
	"$temporary/analyze-constant-negative-zero-clean.time"
expect_status "analyze constant-negative-zero clean test" 0 "$code"
expect_empty "analyze constant-negative-zero clean test stdout" \
	"$temporary/analyze-constant-negative-zero-clean.stdout"
expect_empty "analyze constant-negative-zero clean test stderr" \
	"$temporary/analyze-constant-negative-zero-clean.stderr"

run_timed "$temporary/analyze-url-query-copy-mutation.stdout" \
	"$temporary/analyze-url-query-copy-mutation.stderr" \
	"$temporary/analyze-url-query-copy-mutation.time" \
	"$strider" analyze --only url-query-copy-mutation \
	"$analyze_dir/url_query_copy_mutation.go"
code=$?
record_timing "analyze url-query-copy-mutation true-positive" \
	"$temporary/analyze-url-query-copy-mutation.time"
expect_status "analyze url-query-copy-mutation true-positive test" 1 "$code"
expect_file "analyze url-query-copy-mutation true-positive test" \
	"$analyze_dir/url_query_copy_mutation.expected" \
	"$temporary/analyze-url-query-copy-mutation.stdout"
expect_empty "analyze url-query-copy-mutation true-positive test stderr" \
	"$temporary/analyze-url-query-copy-mutation.stderr"

run_timed "$temporary/analyze-url-query-copy-mutation-clean.stdout" \
	"$temporary/analyze-url-query-copy-mutation-clean.stderr" \
	"$temporary/analyze-url-query-copy-mutation-clean.time" \
	"$strider" analyze --only url-query-copy-mutation \
	"$analyze_dir/url_query_copy_mutation_clean.go"
code=$?
record_timing "analyze url-query-copy-mutation clean" \
	"$temporary/analyze-url-query-copy-mutation-clean.time"
expect_status "analyze url-query-copy-mutation clean test" 0 "$code"
expect_empty "analyze url-query-copy-mutation clean test stdout" \
	"$temporary/analyze-url-query-copy-mutation-clean.stdout"
expect_empty "analyze url-query-copy-mutation clean test stderr" \
	"$temporary/analyze-url-query-copy-mutation-clean.stderr"

run_timed "$temporary/analyze-sort-conversion-without-sort.stdout" \
	"$temporary/analyze-sort-conversion-without-sort.stderr" \
	"$temporary/analyze-sort-conversion-without-sort.time" \
	"$strider" analyze --only sort-conversion-without-sort \
	"$analyze_dir/sort_conversion_without_sort.go"
code=$?
record_timing "analyze sort-conversion-without-sort true-positive" \
	"$temporary/analyze-sort-conversion-without-sort.time"
expect_status "analyze sort-conversion-without-sort true-positive test" 1 "$code"
expect_file "analyze sort-conversion-without-sort true-positive test" \
	"$analyze_dir/sort_conversion_without_sort.expected" \
	"$temporary/analyze-sort-conversion-without-sort.stdout"
expect_empty "analyze sort-conversion-without-sort true-positive test stderr" \
	"$temporary/analyze-sort-conversion-without-sort.stderr"

run_timed "$temporary/analyze-sort-conversion-without-sort-clean.stdout" \
	"$temporary/analyze-sort-conversion-without-sort-clean.stderr" \
	"$temporary/analyze-sort-conversion-without-sort-clean.time" \
	"$strider" analyze --only sort-conversion-without-sort \
	"$analyze_dir/sort_conversion_without_sort_clean.go"
code=$?
record_timing "analyze sort-conversion-without-sort clean" \
	"$temporary/analyze-sort-conversion-without-sort-clean.time"
expect_status "analyze sort-conversion-without-sort clean test" 0 "$code"
expect_empty "analyze sort-conversion-without-sort clean test stdout" \
	"$temporary/analyze-sort-conversion-without-sort-clean.stdout"
expect_empty "analyze sort-conversion-without-sort clean test stderr" \
	"$temporary/analyze-sort-conversion-without-sort-clean.stderr"

run_timed "$temporary/analyze-random-bound-one.stdout" \
	"$temporary/analyze-random-bound-one.stderr" \
	"$temporary/analyze-random-bound-one.time" \
	"$strider" analyze --only random-bound-one \
	"$analyze_dir/random_bound_one.go"
code=$?
record_timing "analyze random-bound-one true-positive" \
	"$temporary/analyze-random-bound-one.time"
expect_status "analyze random-bound-one true-positive test" 1 "$code"
expect_file "analyze random-bound-one true-positive test" \
	"$analyze_dir/random_bound_one.expected" \
	"$temporary/analyze-random-bound-one.stdout"
expect_empty "analyze random-bound-one true-positive test stderr" \
	"$temporary/analyze-random-bound-one.stderr"

run_timed "$temporary/analyze-random-bound-one-clean.stdout" \
	"$temporary/analyze-random-bound-one-clean.stderr" \
	"$temporary/analyze-random-bound-one-clean.time" \
	"$strider" analyze --only random-bound-one \
	"$analyze_dir/random_bound_one_clean.go"
code=$?
record_timing "analyze random-bound-one clean" \
	"$temporary/analyze-random-bound-one-clean.time"
expect_status "analyze random-bound-one clean test" 0 "$code"
expect_empty "analyze random-bound-one clean test stdout" \
	"$temporary/analyze-random-bound-one-clean.stdout"
expect_empty "analyze random-bound-one clean test stderr" \
	"$temporary/analyze-random-bound-one-clean.stderr"

run_timed "$temporary/analyze-never-nil-comparison.stdout" \
	"$temporary/analyze-never-nil-comparison.stderr" \
	"$temporary/analyze-never-nil-comparison.time" \
	"$strider" analyze --only never-nil-comparison \
	"$analyze_dir/never_nil_comparison.go"
code=$?
record_timing "analyze never-nil-comparison true-positive" \
	"$temporary/analyze-never-nil-comparison.time"
expect_status "analyze never-nil-comparison true-positive test" 1 "$code"
expect_file "analyze never-nil-comparison true-positive test" \
	"$analyze_dir/never_nil_comparison.expected" \
	"$temporary/analyze-never-nil-comparison.stdout"
expect_empty "analyze never-nil-comparison true-positive test stderr" \
	"$temporary/analyze-never-nil-comparison.stderr"

run_timed "$temporary/analyze-never-nil-comparison-clean.stdout" \
	"$temporary/analyze-never-nil-comparison-clean.stderr" \
	"$temporary/analyze-never-nil-comparison-clean.time" \
	"$strider" analyze --only never-nil-comparison \
	"$analyze_dir/never_nil_comparison_clean.go"
code=$?
record_timing "analyze never-nil-comparison clean" \
	"$temporary/analyze-never-nil-comparison-clean.time"
expect_status "analyze never-nil-comparison clean test" 0 "$code"
expect_empty "analyze never-nil-comparison clean test stdout" \
	"$temporary/analyze-never-nil-comparison-clean.stdout"
expect_empty "analyze never-nil-comparison clean test stderr" \
	"$temporary/analyze-never-nil-comparison-clean.stderr"

run_timed "$temporary/analyze-impossible-platform-comparison.stdout" \
	"$temporary/analyze-impossible-platform-comparison.stderr" \
	"$temporary/analyze-impossible-platform-comparison.time" \
	env GOOS=darwin GOARCH=arm64 "$strider" analyze --only impossible-platform-comparison \
	"$analyze_dir/impossible_platform_comparison.go"
code=$?
record_timing "analyze impossible-platform-comparison true-positive" \
	"$temporary/analyze-impossible-platform-comparison.time"
expect_status "analyze impossible-platform-comparison true-positive test" 1 "$code"
expect_file "analyze impossible-platform-comparison true-positive test" \
	"$analyze_dir/impossible_platform_comparison.expected" \
	"$temporary/analyze-impossible-platform-comparison.stdout"
expect_empty "analyze impossible-platform-comparison true-positive test stderr" \
	"$temporary/analyze-impossible-platform-comparison.stderr"

run_timed "$temporary/analyze-impossible-platform-comparison-clean.stdout" \
	"$temporary/analyze-impossible-platform-comparison-clean.stderr" \
	"$temporary/analyze-impossible-platform-comparison-clean.time" \
	env GOOS=darwin GOARCH=arm64 "$strider" analyze --only impossible-platform-comparison \
	"$analyze_dir/impossible_platform_comparison_clean.go"
code=$?
record_timing "analyze impossible-platform-comparison clean" \
	"$temporary/analyze-impossible-platform-comparison-clean.time"
expect_status "analyze impossible-platform-comparison clean test" 0 "$code"
expect_empty "analyze impossible-platform-comparison clean test stdout" \
	"$temporary/analyze-impossible-platform-comparison-clean.stdout"
expect_empty "analyze impossible-platform-comparison clean test stderr" \
	"$temporary/analyze-impossible-platform-comparison-clean.stderr"

run_timed "$temporary/analyze-nil-map-assignment.stdout" \
	"$temporary/analyze-nil-map-assignment.stderr" \
	"$temporary/analyze-nil-map-assignment.time" \
	"$strider" analyze --only nil-map-assignment \
	"$analyze_dir/nil_map_assignment.go"
code=$?
record_timing "analyze nil-map-assignment true-positive" \
	"$temporary/analyze-nil-map-assignment.time"
expect_status "analyze nil-map-assignment true-positive test" 1 "$code"
expect_file "analyze nil-map-assignment true-positive test" \
	"$analyze_dir/nil_map_assignment.expected" \
	"$temporary/analyze-nil-map-assignment.stdout"
expect_empty "analyze nil-map-assignment true-positive test stderr" \
	"$temporary/analyze-nil-map-assignment.stderr"

run_timed "$temporary/analyze-nil-map-assignment-clean.stdout" \
	"$temporary/analyze-nil-map-assignment-clean.stderr" \
	"$temporary/analyze-nil-map-assignment-clean.time" \
	"$strider" analyze --only nil-map-assignment \
	"$analyze_dir/nil_map_assignment_clean.go"
code=$?
record_timing "analyze nil-map-assignment clean" \
	"$temporary/analyze-nil-map-assignment-clean.time"
expect_status "analyze nil-map-assignment clean test" 0 "$code"
expect_empty "analyze nil-map-assignment clean test stdout" \
	"$temporary/analyze-nil-map-assignment-clean.stdout"
expect_empty "analyze nil-map-assignment clean test stderr" \
	"$temporary/analyze-nil-map-assignment-clean.stderr"

run_timed "$temporary/analyze-defer-close-before-error-check.stdout" \
	"$temporary/analyze-defer-close-before-error-check.stderr" \
	"$temporary/analyze-defer-close-before-error-check.time" \
	"$strider" analyze --only defer-close-before-error-check \
	"$analyze_dir/defer_close_before_error_check.go"
code=$?
record_timing "analyze defer-close-before-error-check true-positive" \
	"$temporary/analyze-defer-close-before-error-check.time"
expect_status "analyze defer-close-before-error-check true-positive test" 1 "$code"
expect_file "analyze defer-close-before-error-check true-positive test" \
	"$analyze_dir/defer_close_before_error_check.expected" \
	"$temporary/analyze-defer-close-before-error-check.stdout"
expect_empty "analyze defer-close-before-error-check true-positive test stderr" \
	"$temporary/analyze-defer-close-before-error-check.stderr"

run_timed "$temporary/analyze-defer-close-before-error-check-clean.stdout" \
	"$temporary/analyze-defer-close-before-error-check-clean.stderr" \
	"$temporary/analyze-defer-close-before-error-check-clean.time" \
	"$strider" analyze --only defer-close-before-error-check \
	"$analyze_dir/defer_close_before_error_check_clean.go"
code=$?
record_timing "analyze defer-close-before-error-check clean" \
	"$temporary/analyze-defer-close-before-error-check-clean.time"
expect_status "analyze defer-close-before-error-check clean test" 0 "$code"
expect_empty "analyze defer-close-before-error-check clean test stdout" \
	"$temporary/analyze-defer-close-before-error-check-clean.stdout"
expect_empty "analyze defer-close-before-error-check clean test stderr" \
	"$temporary/analyze-defer-close-before-error-check-clean.stderr"

run_timed "$temporary/analyze-spinning-empty-loop.stdout" \
	"$temporary/analyze-spinning-empty-loop.stderr" \
	"$temporary/analyze-spinning-empty-loop.time" \
	"$strider" analyze --only spinning-empty-loop \
	"$analyze_dir/spinning_empty_loop.go"
code=$?
record_timing "analyze spinning-empty-loop true-positive" \
	"$temporary/analyze-spinning-empty-loop.time"
expect_status "analyze spinning-empty-loop true-positive test" 1 "$code"
expect_file "analyze spinning-empty-loop true-positive test" \
	"$analyze_dir/spinning_empty_loop.expected" \
	"$temporary/analyze-spinning-empty-loop.stdout"
expect_empty "analyze spinning-empty-loop true-positive test stderr" \
	"$temporary/analyze-spinning-empty-loop.stderr"

run_timed "$temporary/analyze-spinning-empty-loop-clean.stdout" \
	"$temporary/analyze-spinning-empty-loop-clean.stderr" \
	"$temporary/analyze-spinning-empty-loop-clean.time" \
	"$strider" analyze --only spinning-empty-loop \
	"$analyze_dir/spinning_empty_loop_clean.go"
code=$?
record_timing "analyze spinning-empty-loop clean" \
	"$temporary/analyze-spinning-empty-loop-clean.time"
expect_status "analyze spinning-empty-loop clean test" 0 "$code"
expect_empty "analyze spinning-empty-loop clean test stdout" \
	"$temporary/analyze-spinning-empty-loop-clean.stdout"
expect_empty "analyze spinning-empty-loop clean test stderr" \
	"$temporary/analyze-spinning-empty-loop-clean.stderr"

run_timed "$temporary/analyze-finalizer-captures-object.stdout" \
	"$temporary/analyze-finalizer-captures-object.stderr" \
	"$temporary/analyze-finalizer-captures-object.time" \
	"$strider" analyze --only finalizer-captures-object \
	"$analyze_dir/finalizer_captures_object.go"
code=$?
record_timing "analyze finalizer-captures-object true-positive" \
	"$temporary/analyze-finalizer-captures-object.time"
expect_status "analyze finalizer-captures-object true-positive test" 1 "$code"
expect_file "analyze finalizer-captures-object true-positive test" \
	"$analyze_dir/finalizer_captures_object.expected" \
	"$temporary/analyze-finalizer-captures-object.stdout"
expect_empty "analyze finalizer-captures-object true-positive test stderr" \
	"$temporary/analyze-finalizer-captures-object.stderr"

run_timed "$temporary/analyze-finalizer-captures-object-clean.stdout" \
	"$temporary/analyze-finalizer-captures-object-clean.stderr" \
	"$temporary/analyze-finalizer-captures-object-clean.time" \
	"$strider" analyze --only finalizer-captures-object \
	"$analyze_dir/finalizer_captures_object_clean.go"
code=$?
record_timing "analyze finalizer-captures-object clean" \
	"$temporary/analyze-finalizer-captures-object-clean.time"
expect_status "analyze finalizer-captures-object clean test" 0 "$code"
expect_empty "analyze finalizer-captures-object clean test stdout" \
	"$temporary/analyze-finalizer-captures-object-clean.stdout"
expect_empty "analyze finalizer-captures-object clean test stderr" \
	"$temporary/analyze-finalizer-captures-object-clean.stderr"

run_timed "$temporary/analyze-infinite-recursion.stdout" \
	"$temporary/analyze-infinite-recursion.stderr" \
	"$temporary/analyze-infinite-recursion.time" \
	"$strider" analyze --only infinite-recursion \
	"$analyze_dir/infinite_recursion.go"
code=$?
record_timing "analyze infinite-recursion true-positive" \
	"$temporary/analyze-infinite-recursion.time"
expect_status "analyze infinite-recursion true-positive test" 1 "$code"
expect_file "analyze infinite-recursion true-positive test" \
	"$analyze_dir/infinite_recursion.expected" \
	"$temporary/analyze-infinite-recursion.stdout"
expect_empty "analyze infinite-recursion true-positive test stderr" \
	"$temporary/analyze-infinite-recursion.stderr"

run_timed "$temporary/analyze-infinite-recursion-clean.stdout" \
	"$temporary/analyze-infinite-recursion-clean.stderr" \
	"$temporary/analyze-infinite-recursion-clean.time" \
	"$strider" analyze --only infinite-recursion \
	"$analyze_dir/infinite_recursion_clean.go"
code=$?
record_timing "analyze infinite-recursion clean" \
	"$temporary/analyze-infinite-recursion-clean.time"
expect_status "analyze infinite-recursion clean test" 0 "$code"
expect_empty "analyze infinite-recursion clean test stdout" \
	"$temporary/analyze-infinite-recursion-clean.stdout"
expect_empty "analyze infinite-recursion clean test stderr" \
	"$temporary/analyze-infinite-recursion-clean.stderr"

run_timed "$temporary/analyze-invalid-printf-call.stdout" \
	"$temporary/analyze-invalid-printf-call.stderr" \
	"$temporary/analyze-invalid-printf-call.time" \
	"$strider" analyze --only invalid-printf-call \
	"$analyze_dir/invalid_printf_call.go"
code=$?
record_timing "analyze invalid-printf-call true-positive" \
	"$temporary/analyze-invalid-printf-call.time"
expect_status "analyze invalid-printf-call true-positive test" 1 "$code"
expect_file "analyze invalid-printf-call true-positive test" \
	"$analyze_dir/invalid_printf_call.expected" \
	"$temporary/analyze-invalid-printf-call.stdout"
expect_empty "analyze invalid-printf-call true-positive test stderr" \
	"$temporary/analyze-invalid-printf-call.stderr"

run_timed "$temporary/analyze-invalid-printf-call-clean.stdout" \
	"$temporary/analyze-invalid-printf-call-clean.stderr" \
	"$temporary/analyze-invalid-printf-call-clean.time" \
	"$strider" analyze --only invalid-printf-call \
	"$analyze_dir/invalid_printf_call_clean.go"
code=$?
record_timing "analyze invalid-printf-call clean" \
	"$temporary/analyze-invalid-printf-call-clean.time"
expect_status "analyze invalid-printf-call clean test" 0 "$code"
expect_empty "analyze invalid-printf-call clean test stdout" \
	"$temporary/analyze-invalid-printf-call-clean.stdout"
expect_empty "analyze invalid-printf-call clean test stderr" \
	"$temporary/analyze-invalid-printf-call-clean.stderr"

run_timed "$temporary/analyze-contradictory-interface-assertion.stdout" \
	"$temporary/analyze-contradictory-interface-assertion.stderr" \
	"$temporary/analyze-contradictory-interface-assertion.time" \
	"$strider" analyze --only contradictory-interface-assertion \
	"$analyze_dir/contradictory_interface_assertion.go"
code=$?
record_timing "analyze contradictory-interface-assertion true-positive" \
	"$temporary/analyze-contradictory-interface-assertion.time"
expect_status "analyze contradictory-interface-assertion true-positive test" 1 "$code"
expect_file "analyze contradictory-interface-assertion true-positive test" \
	"$analyze_dir/contradictory_interface_assertion.expected" \
	"$temporary/analyze-contradictory-interface-assertion.stdout"
expect_empty "analyze contradictory-interface-assertion true-positive test stderr" \
	"$temporary/analyze-contradictory-interface-assertion.stderr"

run_timed "$temporary/analyze-contradictory-interface-assertion-clean.stdout" \
	"$temporary/analyze-contradictory-interface-assertion-clean.stderr" \
	"$temporary/analyze-contradictory-interface-assertion-clean.time" \
	"$strider" analyze --only contradictory-interface-assertion \
	"$analyze_dir/contradictory_interface_assertion_clean.go"
code=$?
record_timing "analyze contradictory-interface-assertion clean" \
	"$temporary/analyze-contradictory-interface-assertion-clean.time"
expect_status "analyze contradictory-interface-assertion clean test" 0 "$code"
expect_empty "analyze contradictory-interface-assertion clean test stdout" \
	"$temporary/analyze-contradictory-interface-assertion-clean.stdout"
expect_empty "analyze contradictory-interface-assertion clean test stderr" \
	"$temporary/analyze-contradictory-interface-assertion-clean.stderr"

run_timed "$temporary/analyze-possible-nil-dereference.stdout" \
	"$temporary/analyze-possible-nil-dereference.stderr" \
	"$temporary/analyze-possible-nil-dereference.time" \
	"$strider" analyze --only possible-nil-dereference \
	"$analyze_dir/possible_nil_dereference.go"
code=$?
record_timing "analyze possible-nil-dereference true-positive" \
	"$temporary/analyze-possible-nil-dereference.time"
expect_status "analyze possible-nil-dereference true-positive test" 1 "$code"
expect_file "analyze possible-nil-dereference true-positive test" \
	"$analyze_dir/possible_nil_dereference.expected" \
	"$temporary/analyze-possible-nil-dereference.stdout"
expect_empty "analyze possible-nil-dereference true-positive test stderr" \
	"$temporary/analyze-possible-nil-dereference.stderr"

run_timed "$temporary/analyze-possible-nil-dereference-clean.stdout" \
	"$temporary/analyze-possible-nil-dereference-clean.stderr" \
	"$temporary/analyze-possible-nil-dereference-clean.time" \
	"$strider" analyze --only possible-nil-dereference \
	"$analyze_dir/possible_nil_dereference_clean.go"
code=$?
record_timing "analyze possible-nil-dereference clean" \
	"$temporary/analyze-possible-nil-dereference-clean.time"
expect_status "analyze possible-nil-dereference clean test" 0 "$code"
expect_empty "analyze possible-nil-dereference clean test stdout" \
	"$temporary/analyze-possible-nil-dereference-clean.stdout"
expect_empty "analyze possible-nil-dereference clean test stderr" \
	"$temporary/analyze-possible-nil-dereference-clean.stderr"

run_timed "$temporary/analyze-odd-paired-arguments.stdout" \
	"$temporary/analyze-odd-paired-arguments.stderr" \
	"$temporary/analyze-odd-paired-arguments.time" \
	"$strider" analyze --only odd-paired-arguments \
	"$analyze_dir/odd_paired_arguments.go"
code=$?
record_timing "analyze odd-paired-arguments true-positive" \
	"$temporary/analyze-odd-paired-arguments.time"
expect_status "analyze odd-paired-arguments true-positive test" 1 "$code"
expect_file "analyze odd-paired-arguments true-positive test" \
	"$analyze_dir/odd_paired_arguments.expected" \
	"$temporary/analyze-odd-paired-arguments.stdout"
expect_empty "analyze odd-paired-arguments true-positive test stderr" \
	"$temporary/analyze-odd-paired-arguments.stderr"

run_timed "$temporary/analyze-odd-paired-arguments-clean.stdout" \
	"$temporary/analyze-odd-paired-arguments-clean.stderr" \
	"$temporary/analyze-odd-paired-arguments-clean.time" \
	"$strider" analyze --only odd-paired-arguments \
	"$analyze_dir/odd_paired_arguments_clean.go"
code=$?
record_timing "analyze odd-paired-arguments clean" \
	"$temporary/analyze-odd-paired-arguments-clean.time"
expect_status "analyze odd-paired-arguments clean test" 0 "$code"
expect_empty "analyze odd-paired-arguments clean test stdout" \
	"$temporary/analyze-odd-paired-arguments-clean.stdout"
expect_empty "analyze odd-paired-arguments clean test stderr" \
	"$temporary/analyze-odd-paired-arguments-clean.stderr"

run_timed "$temporary/analyze-regexp-match-in-loop.stdout" \
	"$temporary/analyze-regexp-match-in-loop.stderr" \
	"$temporary/analyze-regexp-match-in-loop.time" \
	"$strider" analyze --only regexp-match-in-loop \
	"$analyze_dir/regexp_match_in_loop.go"
code=$?
record_timing "analyze regexp-match-in-loop true-positive" \
	"$temporary/analyze-regexp-match-in-loop.time"
expect_status "analyze regexp-match-in-loop true-positive test" 1 "$code"
expect_file "analyze regexp-match-in-loop true-positive test" \
	"$analyze_dir/regexp_match_in_loop.expected" \
	"$temporary/analyze-regexp-match-in-loop.stdout"
expect_empty "analyze regexp-match-in-loop true-positive test stderr" \
	"$temporary/analyze-regexp-match-in-loop.stderr"

run_timed "$temporary/analyze-regexp-match-in-loop-clean.stdout" \
	"$temporary/analyze-regexp-match-in-loop-clean.stderr" \
	"$temporary/analyze-regexp-match-in-loop-clean.time" \
	"$strider" analyze --only regexp-match-in-loop \
	"$analyze_dir/regexp_match_in_loop_clean.go"
code=$?
record_timing "analyze regexp-match-in-loop clean" \
	"$temporary/analyze-regexp-match-in-loop-clean.time"
expect_status "analyze regexp-match-in-loop clean test" 0 "$code"
expect_empty "analyze regexp-match-in-loop clean test stdout" \
	"$temporary/analyze-regexp-match-in-loop-clean.stdout"
expect_empty "analyze regexp-match-in-loop clean test stderr" \
	"$temporary/analyze-regexp-match-in-loop-clean.stderr"

run_timed "$temporary/analyze-separate-byte-string-map-key.stdout" \
	"$temporary/analyze-separate-byte-string-map-key.stderr" \
	"$temporary/analyze-separate-byte-string-map-key.time" \
	"$strider" analyze --only separate-byte-string-map-key \
	"$analyze_dir/separate_byte_string_map_key.go"
code=$?
record_timing "analyze separate-byte-string-map-key true-positive" \
	"$temporary/analyze-separate-byte-string-map-key.time"
expect_status "analyze separate-byte-string-map-key true-positive test" 1 "$code"
expect_file "analyze separate-byte-string-map-key true-positive test" \
	"$analyze_dir/separate_byte_string_map_key.expected" \
	"$temporary/analyze-separate-byte-string-map-key.stdout"
expect_empty "analyze separate-byte-string-map-key true-positive test stderr" \
	"$temporary/analyze-separate-byte-string-map-key.stderr"

run_timed "$temporary/analyze-separate-byte-string-map-key-clean.stdout" \
	"$temporary/analyze-separate-byte-string-map-key-clean.stderr" \
	"$temporary/analyze-separate-byte-string-map-key-clean.time" \
	"$strider" analyze --only separate-byte-string-map-key \
	"$analyze_dir/separate_byte_string_map_key_clean.go"
code=$?
record_timing "analyze separate-byte-string-map-key clean" \
	"$temporary/analyze-separate-byte-string-map-key-clean.time"
expect_status "analyze separate-byte-string-map-key clean test" 0 "$code"
expect_empty "analyze separate-byte-string-map-key clean test stdout" \
	"$temporary/analyze-separate-byte-string-map-key-clean.stdout"
expect_empty "analyze separate-byte-string-map-key clean test stderr" \
	"$temporary/analyze-separate-byte-string-map-key-clean.stderr"

run_timed "$temporary/analyze-non-pointer-sync-pool-value.stdout" \
	"$temporary/analyze-non-pointer-sync-pool-value.stderr" \
	"$temporary/analyze-non-pointer-sync-pool-value.time" \
	"$strider" analyze --only non-pointer-sync-pool-value \
	"$analyze_dir/non_pointer_sync_pool_value.go"
code=$?
record_timing "analyze non-pointer-sync-pool-value true-positive" \
	"$temporary/analyze-non-pointer-sync-pool-value.time"
expect_status "analyze non-pointer-sync-pool-value true-positive test" 1 "$code"
expect_file "analyze non-pointer-sync-pool-value true-positive test" \
	"$analyze_dir/non_pointer_sync_pool_value.expected" \
	"$temporary/analyze-non-pointer-sync-pool-value.stdout"
expect_empty "analyze non-pointer-sync-pool-value true-positive test stderr" \
	"$temporary/analyze-non-pointer-sync-pool-value.stderr"

run_timed "$temporary/analyze-non-pointer-sync-pool-value-clean.stdout" \
	"$temporary/analyze-non-pointer-sync-pool-value-clean.stderr" \
	"$temporary/analyze-non-pointer-sync-pool-value-clean.time" \
	"$strider" analyze --only non-pointer-sync-pool-value \
	"$analyze_dir/non_pointer_sync_pool_value_clean.go"
code=$?
record_timing "analyze non-pointer-sync-pool-value clean" \
	"$temporary/analyze-non-pointer-sync-pool-value-clean.time"
expect_status "analyze non-pointer-sync-pool-value clean test" 0 "$code"
expect_empty "analyze non-pointer-sync-pool-value clean test stdout" \
	"$temporary/analyze-non-pointer-sync-pool-value-clean.stdout"
expect_empty "analyze non-pointer-sync-pool-value clean test stderr" \
	"$temporary/analyze-non-pointer-sync-pool-value-clean.stderr"

run_timed "$temporary/analyze-case-insensitive-string-comparison.stdout" \
	"$temporary/analyze-case-insensitive-string-comparison.stderr" \
	"$temporary/analyze-case-insensitive-string-comparison.time" \
	"$strider" analyze --only case-insensitive-string-comparison \
	"$analyze_dir/case_insensitive_string_comparison.go"
code=$?
record_timing "analyze case-insensitive-string-comparison true-positive" \
	"$temporary/analyze-case-insensitive-string-comparison.time"
expect_status "analyze case-insensitive-string-comparison true-positive test" 1 "$code"
expect_file "analyze case-insensitive-string-comparison true-positive test" \
	"$analyze_dir/case_insensitive_string_comparison.expected" \
	"$temporary/analyze-case-insensitive-string-comparison.stdout"
expect_empty "analyze case-insensitive-string-comparison true-positive test stderr" \
	"$temporary/analyze-case-insensitive-string-comparison.stderr"

run_timed "$temporary/analyze-case-insensitive-string-comparison-clean.stdout" \
	"$temporary/analyze-case-insensitive-string-comparison-clean.stderr" \
	"$temporary/analyze-case-insensitive-string-comparison-clean.time" \
	"$strider" analyze --only case-insensitive-string-comparison \
	"$analyze_dir/case_insensitive_string_comparison_clean.go"
code=$?
record_timing "analyze case-insensitive-string-comparison clean" \
	"$temporary/analyze-case-insensitive-string-comparison-clean.time"
expect_status "analyze case-insensitive-string-comparison clean test" 0 "$code"
expect_empty "analyze case-insensitive-string-comparison clean test stdout" \
	"$temporary/analyze-case-insensitive-string-comparison-clean.stdout"
expect_empty "analyze case-insensitive-string-comparison clean test stderr" \
	"$temporary/analyze-case-insensitive-string-comparison-clean.stderr"

run_timed "$temporary/analyze-byte-string-write.stdout" \
	"$temporary/analyze-byte-string-write.stderr" \
	"$temporary/analyze-byte-string-write.time" \
	"$strider" analyze --only byte-string-write \
	"$analyze_dir/byte_string_write.go"
code=$?
record_timing "analyze byte-string-write true-positive" \
	"$temporary/analyze-byte-string-write.time"
expect_status "analyze byte-string-write true-positive test" 1 "$code"
expect_file "analyze byte-string-write true-positive test" \
	"$analyze_dir/byte_string_write.expected" \
	"$temporary/analyze-byte-string-write.stdout"
expect_empty "analyze byte-string-write true-positive test stderr" \
	"$temporary/analyze-byte-string-write.stderr"

run_timed "$temporary/analyze-byte-string-write-clean.stdout" \
	"$temporary/analyze-byte-string-write-clean.stderr" \
	"$temporary/analyze-byte-string-write-clean.time" \
	"$strider" analyze --only byte-string-write \
	"$analyze_dir/byte_string_write_clean.go"
code=$?
record_timing "analyze byte-string-write clean" \
	"$temporary/analyze-byte-string-write-clean.time"
expect_status "analyze byte-string-write clean test" 0 "$code"
expect_empty "analyze byte-string-write clean test stdout" \
	"$temporary/analyze-byte-string-write-clean.stdout"
expect_empty "analyze byte-string-write clean test stderr" \
	"$temporary/analyze-byte-string-write-clean.stderr"

run_timed "$temporary/analyze-decimal-file-mode.stdout" \
	"$temporary/analyze-decimal-file-mode.stderr" \
	"$temporary/analyze-decimal-file-mode.time" \
	"$strider" analyze --only decimal-file-mode \
	"$analyze_dir/decimal_file_mode.go"
code=$?
record_timing "analyze decimal-file-mode true-positive" \
	"$temporary/analyze-decimal-file-mode.time"
expect_status "analyze decimal-file-mode true-positive test" 1 "$code"
expect_file "analyze decimal-file-mode true-positive test" \
	"$analyze_dir/decimal_file_mode.expected" \
	"$temporary/analyze-decimal-file-mode.stdout"
expect_empty "analyze decimal-file-mode true-positive test stderr" \
	"$temporary/analyze-decimal-file-mode.stderr"

run_timed "$temporary/analyze-decimal-file-mode-clean.stdout" \
	"$temporary/analyze-decimal-file-mode-clean.stderr" \
	"$temporary/analyze-decimal-file-mode-clean.time" \
	"$strider" analyze --only decimal-file-mode \
	"$analyze_dir/decimal_file_mode_clean.go"
code=$?
record_timing "analyze decimal-file-mode clean" \
	"$temporary/analyze-decimal-file-mode-clean.time"
expect_status "analyze decimal-file-mode clean test" 0 "$code"
expect_empty "analyze decimal-file-mode clean test stdout" \
	"$temporary/analyze-decimal-file-mode-clean.stdout"
expect_empty "analyze decimal-file-mode clean test stderr" \
	"$temporary/analyze-decimal-file-mode-clean.stderr"

run_timed "$temporary/analyze-partially-typed-constant-group.stdout" \
	"$temporary/analyze-partially-typed-constant-group.stderr" \
	"$temporary/analyze-partially-typed-constant-group.time" \
	"$strider" analyze --only partially-typed-constant-group \
	"$analyze_dir/partially_typed_constant_group.go"
code=$?
record_timing "analyze partially-typed-constant-group true-positive" \
	"$temporary/analyze-partially-typed-constant-group.time"
expect_status "analyze partially-typed-constant-group true-positive test" 1 "$code"
expect_file "analyze partially-typed-constant-group true-positive test" \
	"$analyze_dir/partially_typed_constant_group.expected" \
	"$temporary/analyze-partially-typed-constant-group.stdout"
expect_empty "analyze partially-typed-constant-group true-positive test stderr" \
	"$temporary/analyze-partially-typed-constant-group.stderr"

run_timed "$temporary/analyze-partially-typed-constant-group-clean.stdout" \
	"$temporary/analyze-partially-typed-constant-group-clean.stderr" \
	"$temporary/analyze-partially-typed-constant-group-clean.time" \
	"$strider" analyze --only partially-typed-constant-group \
	"$analyze_dir/partially_typed_constant_group_clean.go"
code=$?
record_timing "analyze partially-typed-constant-group clean" \
	"$temporary/analyze-partially-typed-constant-group-clean.time"
expect_status "analyze partially-typed-constant-group clean test" 0 "$code"
expect_empty "analyze partially-typed-constant-group clean test stdout" \
	"$temporary/analyze-partially-typed-constant-group-clean.stdout"
expect_empty "analyze partially-typed-constant-group clean test stderr" \
	"$temporary/analyze-partially-typed-constant-group-clean.stderr"

run_timed "$temporary/analyze-unexported-serialization-fields.stdout" \
	"$temporary/analyze-unexported-serialization-fields.stderr" \
	"$temporary/analyze-unexported-serialization-fields.time" \
	"$strider" analyze --only unexported-serialization-fields \
	"$analyze_dir/unexported_serialization_fields.go"
code=$?
record_timing "analyze unexported-serialization-fields true-positive" \
	"$temporary/analyze-unexported-serialization-fields.time"
expect_status "analyze unexported-serialization-fields true-positive test" 1 "$code"
expect_file "analyze unexported-serialization-fields true-positive test" \
	"$analyze_dir/unexported_serialization_fields.expected" \
	"$temporary/analyze-unexported-serialization-fields.stdout"
expect_empty "analyze unexported-serialization-fields true-positive test stderr" \
	"$temporary/analyze-unexported-serialization-fields.stderr"

run_timed "$temporary/analyze-unexported-serialization-fields-clean.stdout" \
	"$temporary/analyze-unexported-serialization-fields-clean.stderr" \
	"$temporary/analyze-unexported-serialization-fields-clean.time" \
	"$strider" analyze --only unexported-serialization-fields \
	"$analyze_dir/unexported_serialization_fields_clean.go"
code=$?
record_timing "analyze unexported-serialization-fields clean" \
	"$temporary/analyze-unexported-serialization-fields-clean.time"
expect_status "analyze unexported-serialization-fields clean test" 0 "$code"
expect_empty "analyze unexported-serialization-fields clean test stdout" \
	"$temporary/analyze-unexported-serialization-fields-clean.stdout"
expect_empty "analyze unexported-serialization-fields clean test stderr" \
	"$temporary/analyze-unexported-serialization-fields-clean.stderr"

run_timed "$temporary/analyze-oversized-fixed-width-shift.stdout" \
	"$temporary/analyze-oversized-fixed-width-shift.stderr" \
	"$temporary/analyze-oversized-fixed-width-shift.time" \
	"$strider" analyze --only oversized-fixed-width-shift \
	"$analyze_dir/oversized_fixed_width_shift.go"
code=$?
record_timing "analyze oversized-fixed-width-shift true-positive" \
	"$temporary/analyze-oversized-fixed-width-shift.time"
expect_status "analyze oversized-fixed-width-shift true-positive test" 1 "$code"
expect_file "analyze oversized-fixed-width-shift true-positive test" \
	"$analyze_dir/oversized_fixed_width_shift.expected" \
	"$temporary/analyze-oversized-fixed-width-shift.stdout"
expect_empty "analyze oversized-fixed-width-shift true-positive test stderr" \
	"$temporary/analyze-oversized-fixed-width-shift.stderr"

run_timed "$temporary/analyze-oversized-fixed-width-shift-clean.stdout" \
	"$temporary/analyze-oversized-fixed-width-shift-clean.stderr" \
	"$temporary/analyze-oversized-fixed-width-shift-clean.time" \
	"$strider" analyze --only oversized-fixed-width-shift \
	"$analyze_dir/oversized_fixed_width_shift_clean.go"
code=$?
record_timing "analyze oversized-fixed-width-shift clean" \
	"$temporary/analyze-oversized-fixed-width-shift-clean.time"
expect_status "analyze oversized-fixed-width-shift clean test" 0 "$code"
expect_empty "analyze oversized-fixed-width-shift clean test stdout" \
	"$temporary/analyze-oversized-fixed-width-shift-clean.stdout"
expect_empty "analyze oversized-fixed-width-shift clean test stderr" \
	"$temporary/analyze-oversized-fixed-width-shift-clean.stderr"

run_timed "$temporary/analyze-dangerous-directory-removal.stdout" \
	"$temporary/analyze-dangerous-directory-removal.stderr" \
	"$temporary/analyze-dangerous-directory-removal.time" \
	"$strider" analyze --only dangerous-directory-removal \
	"$analyze_dir/dangerous_directory_removal.go"
code=$?
record_timing "analyze dangerous-directory-removal true-positive" \
	"$temporary/analyze-dangerous-directory-removal.time"
expect_status "analyze dangerous-directory-removal true-positive test" 1 "$code"
expect_file "analyze dangerous-directory-removal true-positive test" \
	"$analyze_dir/dangerous_directory_removal.expected" \
	"$temporary/analyze-dangerous-directory-removal.stdout"
expect_empty "analyze dangerous-directory-removal true-positive test stderr" \
	"$temporary/analyze-dangerous-directory-removal.stderr"

run_timed "$temporary/analyze-dangerous-directory-removal-clean.stdout" \
	"$temporary/analyze-dangerous-directory-removal-clean.stderr" \
	"$temporary/analyze-dangerous-directory-removal-clean.time" \
	"$strider" analyze --only dangerous-directory-removal \
	"$analyze_dir/dangerous_directory_removal_clean.go"
code=$?
record_timing "analyze dangerous-directory-removal clean" \
	"$temporary/analyze-dangerous-directory-removal-clean.time"
expect_status "analyze dangerous-directory-removal clean test" 0 "$code"
expect_empty "analyze dangerous-directory-removal clean test stdout" \
	"$temporary/analyze-dangerous-directory-removal-clean.stdout"
expect_empty "analyze dangerous-directory-removal clean test stderr" \
	"$temporary/analyze-dangerous-directory-removal-clean.stderr"

run_timed "$temporary/analyze-failed-assertion-shadow-read.stdout" \
	"$temporary/analyze-failed-assertion-shadow-read.stderr" \
	"$temporary/analyze-failed-assertion-shadow-read.time" \
	"$strider" analyze --only failed-assertion-shadow-read \
	"$analyze_dir/failed_assertion_shadow_read.go"
code=$?
record_timing "analyze failed-assertion-shadow-read true-positive" \
	"$temporary/analyze-failed-assertion-shadow-read.time"
expect_status "analyze failed-assertion-shadow-read true-positive test" 1 "$code"
expect_file "analyze failed-assertion-shadow-read true-positive test" \
	"$analyze_dir/failed_assertion_shadow_read.expected" \
	"$temporary/analyze-failed-assertion-shadow-read.stdout"
expect_empty "analyze failed-assertion-shadow-read true-positive test stderr" \
	"$temporary/analyze-failed-assertion-shadow-read.stderr"

run_timed "$temporary/analyze-failed-assertion-shadow-read-clean.stdout" \
	"$temporary/analyze-failed-assertion-shadow-read-clean.stderr" \
	"$temporary/analyze-failed-assertion-shadow-read-clean.time" \
	"$strider" analyze --only failed-assertion-shadow-read \
	"$analyze_dir/failed_assertion_shadow_read_clean.go"
code=$?
record_timing "analyze failed-assertion-shadow-read clean" \
	"$temporary/analyze-failed-assertion-shadow-read-clean.time"
expect_status "analyze failed-assertion-shadow-read clean test" 0 "$code"
expect_empty "analyze failed-assertion-shadow-read clean test stdout" \
	"$temporary/analyze-failed-assertion-shadow-read-clean.stdout"
expect_empty "analyze failed-assertion-shadow-read clean test stderr" \
	"$temporary/analyze-failed-assertion-shadow-read-clean.stderr"

run_timed "$temporary/analyze-deferred-return-function-not-called.stdout" \
	"$temporary/analyze-deferred-return-function-not-called.stderr" \
	"$temporary/analyze-deferred-return-function-not-called.time" \
	"$strider" analyze --only deferred-return-function-not-called \
	"$analyze_dir/deferred_return_function_not_called.go"
code=$?
record_timing "analyze deferred-return-function-not-called true-positive" \
	"$temporary/analyze-deferred-return-function-not-called.time"
expect_status "analyze deferred-return-function-not-called true-positive test" 1 "$code"
expect_file "analyze deferred-return-function-not-called true-positive test" \
	"$analyze_dir/deferred_return_function_not_called.expected" \
	"$temporary/analyze-deferred-return-function-not-called.stdout"
expect_empty "analyze deferred-return-function-not-called true-positive test stderr" \
	"$temporary/analyze-deferred-return-function-not-called.stderr"

run_timed "$temporary/analyze-deferred-return-function-not-called-clean.stdout" \
	"$temporary/analyze-deferred-return-function-not-called-clean.stderr" \
	"$temporary/analyze-deferred-return-function-not-called-clean.time" \
	"$strider" analyze --only deferred-return-function-not-called \
	"$analyze_dir/deferred_return_function_not_called_clean.go"
code=$?
record_timing "analyze deferred-return-function-not-called clean" \
	"$temporary/analyze-deferred-return-function-not-called-clean.time"
expect_status "analyze deferred-return-function-not-called clean test" 0 "$code"
expect_empty "analyze deferred-return-function-not-called clean test stdout" \
	"$temporary/analyze-deferred-return-function-not-called-clean.stdout"
expect_empty "analyze deferred-return-function-not-called clean test stderr" \
	"$temporary/analyze-deferred-return-function-not-called-clean.stderr"

run_timed "$temporary/analyze-duration-multiplied-by-duration.stdout" \
	"$temporary/analyze-duration-multiplied-by-duration.stderr" \
	"$temporary/analyze-duration-multiplied-by-duration.time" \
	"$strider" analyze --only duration-multiplied-by-duration \
	"$analyze_dir/duration_multiplied_by_duration.go"
code=$?
record_timing "analyze duration-multiplied-by-duration true-positive" \
	"$temporary/analyze-duration-multiplied-by-duration.time"
expect_status "analyze duration-multiplied-by-duration true-positive test" 1 "$code"
expect_file "analyze duration-multiplied-by-duration true-positive test" \
	"$analyze_dir/duration_multiplied_by_duration.expected" \
	"$temporary/analyze-duration-multiplied-by-duration.stdout"
expect_empty "analyze duration-multiplied-by-duration true-positive test stderr" \
	"$temporary/analyze-duration-multiplied-by-duration.stderr"

run_timed "$temporary/analyze-duration-multiplied-by-duration-clean.stdout" \
	"$temporary/analyze-duration-multiplied-by-duration-clean.stderr" \
	"$temporary/analyze-duration-multiplied-by-duration-clean.time" \
	"$strider" analyze --only duration-multiplied-by-duration \
	"$analyze_dir/duration_multiplied_by_duration_clean.go"
code=$?
record_timing "analyze duration-multiplied-by-duration clean" \
	"$temporary/analyze-duration-multiplied-by-duration-clean.time"
expect_status "analyze duration-multiplied-by-duration clean test" 0 "$code"
expect_empty "analyze duration-multiplied-by-duration clean test stdout" \
	"$temporary/analyze-duration-multiplied-by-duration-clean.stdout"
expect_empty "analyze duration-multiplied-by-duration clean test stderr" \
	"$temporary/analyze-duration-multiplied-by-duration-clean.stderr"

run_timed "$temporary/analyze-context-stored-in-struct.stdout" \
	"$temporary/analyze-context-stored-in-struct.stderr" \
	"$temporary/analyze-context-stored-in-struct.time" \
	"$strider" analyze --only context-stored-in-struct \
	"$analyze_dir/context_stored_in_struct.go"
code=$?
record_timing "analyze context-stored-in-struct true-positive" \
	"$temporary/analyze-context-stored-in-struct.time"
expect_status "analyze context-stored-in-struct true-positive test" 1 "$code"
expect_file "analyze context-stored-in-struct true-positive test" \
	"$analyze_dir/context_stored_in_struct.expected" \
	"$temporary/analyze-context-stored-in-struct.stdout"
expect_empty "analyze context-stored-in-struct true-positive test stderr" \
	"$temporary/analyze-context-stored-in-struct.stderr"

run_timed "$temporary/analyze-context-stored-in-struct-clean.stdout" \
	"$temporary/analyze-context-stored-in-struct-clean.stderr" \
	"$temporary/analyze-context-stored-in-struct-clean.time" \
	"$strider" analyze --only context-stored-in-struct \
	"$analyze_dir/context_stored_in_struct_clean.go"
code=$?
record_timing "analyze context-stored-in-struct clean" \
	"$temporary/analyze-context-stored-in-struct-clean.time"
expect_status "analyze context-stored-in-struct clean test" 0 "$code"
expect_empty "analyze context-stored-in-struct clean test stdout" \
	"$temporary/analyze-context-stored-in-struct-clean.stdout"
expect_empty "analyze context-stored-in-struct clean test stderr" \
	"$temporary/analyze-context-stored-in-struct-clean.stderr"

run_timed "$temporary/analyze-unsafe-formatted-url-host-port.stdout" \
	"$temporary/analyze-unsafe-formatted-url-host-port.stderr" \
	"$temporary/analyze-unsafe-formatted-url-host-port.time" \
	"$strider" analyze --only unsafe-formatted-url-host-port \
	"$analyze_dir/unsafe_formatted_url_host_port.go"
code=$?
record_timing "analyze unsafe-formatted-url-host-port true-positive" \
	"$temporary/analyze-unsafe-formatted-url-host-port.time"
expect_status "analyze unsafe-formatted-url-host-port true-positive test" 1 "$code"
expect_file "analyze unsafe-formatted-url-host-port true-positive test" \
	"$analyze_dir/unsafe_formatted_url_host_port.expected" \
	"$temporary/analyze-unsafe-formatted-url-host-port.stdout"
expect_empty "analyze unsafe-formatted-url-host-port true-positive test stderr" \
	"$temporary/analyze-unsafe-formatted-url-host-port.stderr"

run_timed "$temporary/analyze-unsafe-formatted-url-host-port-clean.stdout" \
	"$temporary/analyze-unsafe-formatted-url-host-port-clean.stderr" \
	"$temporary/analyze-unsafe-formatted-url-host-port-clean.time" \
	"$strider" analyze --only unsafe-formatted-url-host-port \
	"$analyze_dir/unsafe_formatted_url_host_port_clean.go"
code=$?
record_timing "analyze unsafe-formatted-url-host-port clean" \
	"$temporary/analyze-unsafe-formatted-url-host-port-clean.time"
expect_status "analyze unsafe-formatted-url-host-port clean test" 0 "$code"
expect_empty "analyze unsafe-formatted-url-host-port clean test stdout" \
	"$temporary/analyze-unsafe-formatted-url-host-port-clean.stdout"
expect_empty "analyze unsafe-formatted-url-host-port clean test stderr" \
	"$temporary/analyze-unsafe-formatted-url-host-port-clean.stderr"

run_timed "$temporary/analyze-unchecked-rows-error.stdout" \
	"$temporary/analyze-unchecked-rows-error.stderr" \
	"$temporary/analyze-unchecked-rows-error.time" \
	"$strider" analyze --only unchecked-rows-error \
	"$analyze_dir/unchecked_rows_error.go"
code=$?
record_timing "analyze unchecked-rows-error true-positive" \
	"$temporary/analyze-unchecked-rows-error.time"
expect_status "analyze unchecked-rows-error true-positive test" 1 "$code"
expect_file "analyze unchecked-rows-error true-positive test" \
	"$analyze_dir/unchecked_rows_error.expected" \
	"$temporary/analyze-unchecked-rows-error.stdout"
expect_empty "analyze unchecked-rows-error true-positive test stderr" \
	"$temporary/analyze-unchecked-rows-error.stderr"

run_timed "$temporary/analyze-unchecked-rows-error-clean.stdout" \
	"$temporary/analyze-unchecked-rows-error-clean.stderr" \
	"$temporary/analyze-unchecked-rows-error-clean.time" \
	"$strider" analyze --only unchecked-rows-error \
	"$analyze_dir/unchecked_rows_error_clean.go"
code=$?
record_timing "analyze unchecked-rows-error clean" \
	"$temporary/analyze-unchecked-rows-error-clean.time"
expect_status "analyze unchecked-rows-error clean test" 0 "$code"
expect_empty "analyze unchecked-rows-error clean test stdout" \
	"$temporary/analyze-unchecked-rows-error-clean.stdout"
expect_empty "analyze unchecked-rows-error clean test stderr" \
	"$temporary/analyze-unchecked-rows-error-clean.stderr"

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
