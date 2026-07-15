---
title: confusing-naming
description: detect names differing only by capitalization.
---

Purpose: detect names differing only by capitalization.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

methods and fields. The rule is part of Strider's extended catalog and runs
when selected with `--only confusing-naming` or when the complete catalog is enabled
with `--all-rules`.
