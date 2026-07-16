---
title: blank-imports
description: require blank imports to be justified.
---

Purpose: require blank imports to be justified.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

main and test packages exempt. The rule is part of Strider's extended catalog and runs
when selected with `--only blank-imports` or when the complete catalog is enabled
with `--all-rules`.
