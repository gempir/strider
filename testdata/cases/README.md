# Curated test cases

These focused cases are Strider's correctness oracle. Add a case here only
after reviewing a Wilds observation and deciding what Strider should do.

- `format/` pairs unformatted input with intentional formatter output.
- `lint/` contains known findings, their exact diagnostics, and clean input
  that must not produce diagnostics.
- `analyze/` contains package-aware static-analysis findings and clean input.

Wilds baselines belong under `testdata/wilds/baselines/`. They detect behavior
changes but do not, by themselves, declare a finding correct or incorrect.
