# Phase 5 dependency and ownership decisions

## Workspace lifetime

A workspace generation is caller-owned and reusable until `Close`. Check and
format runners no longer release file caches behind the caller's back.
`Close` releases source/CST caches and is idempotent; byte, CST, and identity
access after closure fails explicitly.

## Unified-diff implementation

The existing `pmezard/go-difflib` transitive module was evaluated as the
smallest available replacement for the local line diff. Its API operates on
normalized text lines and owns unified-header rendering, while Strider's
tested contract preserves CRLF state, missing-final-newline markers, exact
three-line hunk merging, and colored rendering. Retaining those contracts
would keep most of the current adapter and hunk code, so making that module a
direct dependency would not delete substantially more code than it adds.
The focused Myers implementation remains behind reconstruction tests and a
fuzz target.

## Module parsing

The formatter now uses `golang.org/x/mod/modfile.ModulePath` instead of a
handwritten line scanner. `x/mod` was already in the module graph and is now a
direct dependency because production imports it. The pinned module is
Go-project maintained under its existing BSD-style license. `go mod verify`,
`go mod tidy -diff`, and the CI-pinned `govulncheck@v1.6.0` all pass; the scan
reports no reachable vulnerabilities.
