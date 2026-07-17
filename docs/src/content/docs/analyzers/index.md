---
title: Analyzers
description: Package-aware correctness and data-flow checks in Strider.
sidebar:
  order: 0
---

`strider analyze` complements the fast CST linter. It deliberately keeps Go's
AST as its syntax model, loads complete packages, resolves types, and constructs
SSA so checks can follow constants, calls, and control flow.

Use analyzers for correctness questions that depend on what an identifier or
call resolves to, how values flow, or which Go and API contracts apply. Use
the [linter](/linter/) for fast, file-local style and maintainability checks.

```sh
strider analyze ./...
strider analyze --only invalid-regexp ./...
strider analyze --format json ./...
strider analyze --format html ./... > analysis-report.html
```

Use `--list-rules` to see the implemented registry and `--explain CODE` for
the built-in summary and examples. Analyzer codes are descriptive kebab-case
names.

## Configuration and baselines

All analyzers are enabled by default. Every analyzer can be disabled, assigned
`note`, `warning`, or `error` severity, and excluded from selected paths:

```toml
[analyzer]
baseline = "analysis-baseline.toml"

[analyzer.rules.deprecated-api-usage]
enabled = false

[analyzer.rules.possible-nil-dereference]
severity = "error"
excludes = ["internal/legacy/**"]
```

Generate a separate analysis baseline with:

```sh
strider analyze --generate-baseline --baseline analysis-baseline.toml ./...
```

See [Configuration](/configuration/#analyzer) and
[Baselines](/baselines/) for the complete behavior.

## Available analyzers

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
