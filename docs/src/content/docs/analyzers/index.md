---
title: Correctness and safety checks
description: Reference for Strider's package-aware correctness and data-flow checks.
sidebar:
  order: 0
---

These 110 checks answer correctness questions that depend on what an identifier
or call resolves to, how values flow, or which Go and API contracts apply. They
all belong to the same `strider check` catalog as formatting and
[style and maintainability checks](/lints/).

Strider chooses and shares the program information required by the selected
codes. Parsing, type resolution, and control-flow construction are internal
capabilities rather than separate user-facing categories.

```sh
strider check ./...
strider check --only invalid-regexp ./...
strider check --format json ./...
strider check --format html ./... > check-report.html
```

Use `--list-checks` to inspect the effective registry and `--explain CODE` for
the built-in summary and examples. Check codes are descriptive kebab-case names.

## Configuration and baselines

All 110 checks in this group are enabled by default. Every check can be disabled,
assigned `note`, `warning`, or `error` severity, and excluded from selected
paths:

```toml
[checks]
baseline = "strider-baseline.toml"

[checks.rules.deprecated-api-usage]
enabled = false

[checks.rules.possible-nil-dereference]
severity = "error"
excludes = ["internal/legacy/**"]
```

Generate the unified baseline with:

```sh
strider check --generate-baseline --baseline strider-baseline.toml ./...
```

See [Configuration](/configuration/#checks) and
[Baselines](/baselines/) for the complete behavior.

## Available checks

- [`invalid-regexp`](./invalid-regexp/)
- [`invalid-template`](./invalid-template/)
- [`invalid-time-layout`](./invalid-time-layout/)
- [`unsupported-binary-write`](./unsupported-binary-write/)
- [`suspicious-sleep`](./suspicious-sleep/)
- [`invalid-exec-command`](./invalid-exec-command/)
- [`dynamic-printf`](./dynamic-printf/)
- [`invalid-url`](./invalid-url/)
- [`non-canonical-header`](./non-canonical-header/)
- [`regexp-find-all-zero`](./regexp-find-all-zero/)
- [`invalid-utf8`](./invalid-utf8/)
- [`nil-context`](./nil-context/)
- [`swapped-seek-arguments`](./swapped-seek-arguments/)
- [`non-pointer-unmarshal`](./non-pointer-unmarshal/)
- [`leaky-time-tick`](./leaky-time-tick/)
- [`untrappable-signal`](./untrappable-signal/)
- [`unbuffered-signal-channel`](./unbuffered-signal-channel/)
- [`zero-replacement-limit`](./zero-replacement-limit/)
- [`deprecated-api-usage`](./deprecated-api-usage/)
- [`invalid-listen-address`](./invalid-listen-address/)
- [`ip-byte-comparison`](./ip-byte-comparison/)
- [`writer-buffer-mutation`](./writer-buffer-mutation/)
- [`duplicate-trim-cutset`](./duplicate-trim-cutset/)
- [`timer-reset-drain-race`](./timer-reset-drain-race/)
- [`unsupported-marshal-type`](./unsupported-marshal-type/)
- [`misaligned-atomic-64`](./misaligned-atomic-64/)
- [`sort-non-slice`](./sort-non-slice/)
- [`context-key-type`](./context-key-type/)
- [`invalid-strconv-argument`](./invalid-strconv-argument/)
- [`overlapping-encode-buffers`](./overlapping-encode-buffers/)
- [`swapped-errors-is-arguments`](./swapped-errors-is-arguments/)
- [`waitgroup-add-inside-goroutine`](./waitgroup-add-inside-goroutine/)
- [`empty-critical-section`](./empty-critical-section/)
- [`testing-fatal-in-goroutine`](./testing-fatal-in-goroutine/)
- [`deferred-lock-after-lock`](./deferred-lock-after-lock/)
- [`testmain-missing-exit`](./testmain-missing-exit/)
- [`benchmark-iteration-mutation`](./benchmark-iteration-mutation/)
- [`identical-binary-operands`](./identical-binary-operands/)
- [`impossible-integer-comparison`](./impossible-integer-comparison/)
- [`single-iteration-loop`](./single-iteration-loop/)
- [`ineffective-value-receiver-assignment`](./ineffective-value-receiver-assignment/)
- [`overwritten-before-use`](./overwritten-before-use/)
- [`unchanged-loop-condition`](./unchanged-loop-condition/)
- [`argument-overwritten-before-use`](./argument-overwritten-before-use/)
- [`unused-append-result`](./unused-append-result/)
- [`nan-comparison`](./nan-comparison/)
- [`pointless-integer-math`](./pointless-integer-math/)
- [`ineffective-bitwise-zero`](./ineffective-bitwise-zero/)
- [`discarded-pure-result`](./discarded-pure-result/)
- [`self-assignment`](./self-assignment/)
- [`unreachable-type-switch-case`](./unreachable-type-switch-case/)
- [`single-argument-append`](./single-argument-append/)
- [`address-nil-comparison`](./address-nil-comparison/)
- [`impossible-interface-nil-comparison`](./impossible-interface-nil-comparison/)
- [`negative-length-capacity-comparison`](./negative-length-capacity-comparison/)
- [`constant-negative-zero`](./constant-negative-zero/)
- [`url-query-copy-mutation`](./url-query-copy-mutation/)
- [`sort-conversion-without-sort`](./sort-conversion-without-sort/)
- [`random-bound-one`](./random-bound-one/)
- [`never-nil-comparison`](./never-nil-comparison/)
- [`impossible-platform-comparison`](./impossible-platform-comparison/)
- [`nil-map-assignment`](./nil-map-assignment/)
- [`defer-close-before-error-check`](./defer-close-before-error-check/)
- [`spinning-empty-loop`](./spinning-empty-loop/)
- [`finalizer-captures-object`](./finalizer-captures-object/)
- [`infinite-recursion`](./infinite-recursion/)
- [`invalid-printf-call`](./invalid-printf-call/)
- [`contradictory-interface-assertion`](./contradictory-interface-assertion/)
- [`possible-nil-dereference`](./possible-nil-dereference/)
- [`odd-paired-arguments`](./odd-paired-arguments/)
- [`regexp-match-in-loop`](./regexp-match-in-loop/)
- [`separate-byte-string-map-key`](./separate-byte-string-map-key/)
- [`non-pointer-sync-pool-value`](./non-pointer-sync-pool-value/)
- [`case-insensitive-string-comparison`](./case-insensitive-string-comparison/)
- [`byte-string-write`](./byte-string-write/)
- [`decimal-file-mode`](./decimal-file-mode/)
- [`partially-typed-constant-group`](./partially-typed-constant-group/)
- [`unexported-serialization-fields`](./unexported-serialization-fields/)
- [`oversized-fixed-width-shift`](./oversized-fixed-width-shift/)
- [`dangerous-directory-removal`](./dangerous-directory-removal/)
- [`failed-assertion-shadow-read`](./failed-assertion-shadow-read/)
- [`deferred-return-function-not-called`](./deferred-return-function-not-called/)
- [`duration-multiplied-by-duration`](./duration-multiplied-by-duration/)
- [`context-stored-in-struct`](./context-stored-in-struct/)
- [`unsafe-formatted-url-host-port`](./unsafe-formatted-url-host-port/)
- [`unchecked-rows-error`](./unchecked-rows-error/)
- [`nil-error-return`](./nil-error-return/)
- [`nil-value-with-nil-error`](./nil-value-with-nil-error/)
- [`unclosed-http-response-body`](./unclosed-http-response-body/)
- [`unclosed-sql-resource`](./unclosed-sql-resource/)
- [`context-cancel-in-loop`](./context-cancel-in-loop/)
- [`copy-lock-value`](./copy-lock-value/)
- [`append-to-sized-slice`](./append-to-sized-slice/)
- [`redundant-conversion`](./redundant-conversion/)
- [`slice-preallocation`](./slice-preallocation/)
- [`inefficient-sprintf`](./inefficient-sprintf/)
- [`interface-method-limit`](./interface-method-limit/)
- [`interface-return`](./interface-return/)
- [`slog-argument-shape`](./slog-argument-shape/)
- [`external-call-in-loop`](./external-call-in-loop/)
- [`excessive-blank-identifiers`](./excessive-blank-identifiers/)
- [`task-comment`](./task-comment/)
- [`doc-comment-period`](./doc-comment-period/)
- [`error-type-naming`](./error-type-naming/)
- [`standard-http-method-constant`](./standard-http-method-constant/)
- [`weak-cryptography`](./weak-cryptography/)
- [`discarded-error-result`](./discarded-error-result/)
- [`inline-error-declaration`](./inline-error-declaration/)
- [`test-parallelism`](./test-parallelism/)
- [`declaration-order`](./declaration-order/)
