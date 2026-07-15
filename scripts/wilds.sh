#!/bin/sh

set -u
export LC_ALL=C

die() {
	echo "error: $*" >&2
	exit 2
}

if test "$#" -lt 5; then
	die "usage: wilds.sh MODE STRIDER WILDS_DIR BASELINES_DIR PROJECT..."
fi

mode=$1
strider=$2
wilds_dir=$3
baselines_dir=$4
shift 4

case "$mode" in
clone|smoke|check|accept) ;;
*) die "unknown mode: $mode" ;;
esac

root=$(pwd -P)
case "$wilds_dir" in
/*) ;;
*) wilds_dir="$root/${wilds_dir#./}" ;;
esac
case "$baselines_dir" in
/*) ;;
*) baselines_dir="$root/${baselines_dir#./}" ;;
esac
if test "$mode" != clone; then
	case "$strider" in
	/*) ;;
	*) strider="$root/${strider#./}" ;;
	esac
	test -x "$strider" || die "$strider is not executable"
fi

if test "$mode" != clone; then
	format_budget=${WILDS_FMT_MAX_SECONDS:-2.0}
	lint_budget=${WILDS_LINT_MAX_SECONDS:-2.0}
	timings_file=${TIMINGS_FILE:-target/timings/wilds-$mode.tsv}
	case "$timings_file" in
	/*) ;;
	*) timings_file="$root/${timings_file#./}" ;;
	esac
	mkdir -p "$(dirname "$timings_file")" || exit 2
	printf 'suite\tproject\toperation\tseconds\tbudget_seconds\tbudget_result\n' > "$timings_file"
	total_seconds=0
	lint_args=${STRIDER_LINT_ARGS:-}
	skip_format=${STRIDER_SKIP_FORMAT:-0}
	suite_name=${STRIDER_SUITE_NAME:-wilds-$mode}
	if test -n "${GITHUB_STEP_SUMMARY:-}"; then
		{
			printf '### Wilds %s timings\n\n' "$mode"
			echo "| Project | Operation | Time | Budget | Result |"
			echo "| --- | --- | ---: | ---: | --- |"
		} >> "$GITHUB_STEP_SUMMARY"
	fi
fi

parse_project() {
	spec=$1
	case "$spec" in
	*,*,*) ;;
	*) die "invalid project specification: $spec" ;;
	esac
	name=${spec%%,*}
	remainder=${spec#*,}
	repository=${remainder%%,*}
	revision=${remainder#*,}
	test -n "$name" || die "project name is empty in: $spec"
	test -n "$repository" || die "repository is empty in: $spec"
	test -n "$revision" || die "revision is empty in: $spec"
	case "$name" in
	*[!A-Za-z0-9._-]*) die "unsafe project name: $name" ;;
	esac
	project="$wilds_dir/$name"
}

clone_project() {
	mkdir -p "$wilds_dir" || die "cannot create $wilds_dir"
	if test -d "$project/.git"; then
		:
	elif test -e "$project"; then
		die "$project exists but is not a Git checkout"
	else
		git clone --filter=blob:none "$repository" "$project" || exit 2
	fi

	if ! git -C "$project" cat-file -e "$revision^{commit}" 2>/dev/null; then
		git -C "$project" fetch --depth 1 origin "$revision" || exit 2
	fi
	resolved=$(git -C "$project" rev-parse "$revision^{commit}") || exit 2
	current=$(git -C "$project" rev-parse HEAD 2>/dev/null || true)
	branch=$(git -C "$project" symbolic-ref -q HEAD 2>/dev/null || true)
	if test "$current" != "$resolved" || test -n "$branch"; then
		if test -n "$(git -C "$project" status --porcelain)"; then
			die "$project has local changes; refusing to switch revisions"
		fi
		git -C "$project" checkout --detach --quiet "$resolved" || exit 2
	fi
	printf 'Using %s at %s\n' "$name" "$resolved"
}

normalize_output() {
	input_file=$1
	output_file=$2
	awk -v prefix="$project/" '
		{
			line = $0
			while ((position = index(line, prefix)) != 0) {
				line = substr(line, 1, position - 1) \
					substr(line, position + length(prefix))
			}
			print line
		}
	' "$input_file" > "$output_file"
}

capture_project() {
	output_dir=$1
	mkdir -p "$output_dir" || die "cannot create $output_dir"
	printf '%s\n' "$revision" > "$output_dir/revision"
	(
		cd "$project" || exit 2
		if test "$skip_format" = 1; then
			: > "$output_dir/format.stdout.raw"
			: > "$output_dir/format.stderr.raw"
			printf 'real 0.00\nuser 0.00\nsys 0.00\n' > "$output_dir/format.time"
			printf '0\n' > "$output_dir/format.status"
		else
			{ time -p "$strider" fmt --diff . \
				> "$output_dir/format.stdout.raw" \
				2> "$output_dir/format.stderr.raw"; } \
				2> "$output_dir/format.time"
			printf '%s\n' "$?" > "$output_dir/format.status"
		fi
		normalize_output "$output_dir/format.stdout.raw" "$output_dir/format.stdout"
		normalize_output "$output_dir/format.stderr.raw" "$output_dir/format.stderr"
		git hash-object --stdin \
			< "$output_dir/format.stdout" \
			> "$output_dir/format.digest"
		# lint_args is an intentionally word-split list of trusted harness flags.
		# shellcheck disable=SC2086
		{ time -p "$strider" lint $lint_args . \
			> "$output_dir/lint.stdout.raw" \
			2> "$output_dir/lint.stderr.raw"; } \
			2> "$output_dir/lint.time"
		printf '%s\n' "$?" > "$output_dir/lint.status"
		normalize_output "$output_dir/lint.stdout.raw" "$output_dir/lint.stdout"
		normalize_output "$output_dir/lint.stderr.raw" "$output_dir/lint.stderr"
		git hash-object --stdin \
			< "$output_dir/lint.stdout" \
			> "$output_dir/lint.digest"
		awk '
			{
				total++
				opening = index($0, "[")
				remaining = substr($0, opening + 1)
				closing = index(remaining, "]")
				if (opening != 0 && closing != 0) {
					code = substr(remaining, 1, closing - 1)
					counts[code]++
				}
			}
			END {
				for (code in counts) {
					printf "%s\t%d\n", code, counts[code]
				}
				printf "TOTAL\t%d\n", total
			}
		' "$output_dir/lint.stdout" | sort > "$output_dir/lint.summary"
	)
}

record_timing() {
	project_name=$1
	operation=$2
	time_file=$3
	budget=$4
	seconds=$(awk '$1 == "real" { print $2; exit }' "$time_file")
	test -n "$seconds" || {
		echo "error: no timing recorded for $project_name strider $operation" >&2
		failed=1
		seconds=0
	}
	total_seconds=$(awk -v total="$total_seconds" -v current="$seconds" \
		'BEGIN { printf "%.2f", total + current }')
	if awk -v actual="$seconds" -v maximum="$budget" \
		'BEGIN { exit !(actual <= maximum) }'
	then
		budget_result=PASS
	else
		budget_result=FAIL
		echo "error: $project_name strider $operation took ${seconds}s; budget is ${budget}s" >&2
		failed=1
	fi
	printf 'Timing: %s / %s: %ss (budget %ss) [%s]\n' \
		"$project_name" "$operation" "$seconds" "$budget" "$budget_result"
	printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
		"$suite_name" "$project_name" "$operation" "$seconds" "$budget" "$budget_result" \
		>> "$timings_file"
	if test -n "${GITHUB_STEP_SUMMARY:-}"; then
		printf '| %s | %s | %ss | %ss | %s |\n' \
			"$project_name" "$operation" "$seconds" "$budget" "$budget_result" \
			>> "$GITHUB_STEP_SUMMARY"
	fi
}

show_observation() {
	label=$1
	output_dir=$2
	command=$label
	if test "$label" = format; then
		command=fmt
	fi
	printf '\n==> %s: strider %s\n' "$name" "$command"
	cat "$output_dir/$label.stdout"
	cat "$output_dir/$label.stderr" >&2
	code=$(cat "$output_dir/$label.status")
	if test "$code" -gt 1; then
		echo "error: $name strider $command exited $code" >&2
		return 1
	fi
	return 0
}

compare_baseline() {
	expected_dir=$1
	actual_dir=$2
	comparison_failed=0
	for artifact in \
		revision \
		format.status format.digest format.stderr \
		lint.status lint.digest lint.summary lint.stderr
	do
		if test ! -f "$expected_dir/$artifact"; then
			echo "error: missing baseline $expected_dir/$artifact" >&2
			comparison_failed=1
		elif ! diff -u "$expected_dir/$artifact" "$actual_dir/$artifact"; then
			comparison_failed=1
			if test "$artifact" = format.digest; then
				echo "Current formatter diff for $name:" >&2
				cat "$actual_dir/format.stdout" >&2
			elif test "$artifact" = lint.digest; then
				echo "Current lint findings for $name:" >&2
				cat "$actual_dir/lint.stdout" >&2
			fi
		fi
	done
	return "$comparison_failed"
}

if test "$mode" = clone; then
	for project_spec in "$@"; do
		parse_project "$project_spec"
		clone_project
	done
	exit 0
fi

temporary=$(mktemp -d "${TMPDIR:-/tmp}/strider-wilds.XXXXXX") || exit 2
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
failed=0

for project_spec in "$@"; do
	parse_project "$project_spec"
	actual="$temporary/$name"
	capture_project "$actual"
	record_timing "$name" fmt "$actual/format.time" "$format_budget"
	record_timing "$name" lint "$actual/lint.time" "$lint_budget"

	case "$mode" in
	smoke)
		show_observation format "$actual" || failed=1
		show_observation lint "$actual" || failed=1
		;;
	check)
		printf '\n==> %s: comparing reviewed baseline\n' "$name"
		compare_baseline "$baselines_dir/$name" "$actual" || failed=1
		;;
	accept)
		destination="$baselines_dir/$name"
		mkdir -p "$destination" || exit 2
		for artifact in \
			revision \
			format.status format.digest format.stderr \
			lint.status lint.digest lint.summary lint.stderr
		do
			cp "$actual/$artifact" "$destination/$artifact" || exit 2
		done
		printf 'Accepted Wilds baseline for %s at %s\n' "$name" "$revision"
		;;
	esac
done

if test "$mode" = check && test "$failed" -eq 0; then
	echo "Wilds baselines match."
fi
printf 'Timing: Wilds %s total Strider time: %ss\n' "$mode" "$total_seconds"
printf '%s\t-\ttotal\t%s\t\tINFO\n' \
	"$suite_name" "$total_seconds" >> "$timings_file"
if test -n "${GITHUB_STEP_SUMMARY:-}"; then
	printf '| **Total** |  | **%ss** |  | INFO |\n' \
		"$total_seconds" >> "$GITHUB_STEP_SUMMARY"
fi
exit "$failed"
