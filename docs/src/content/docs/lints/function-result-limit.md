---
title: function-result-limit
description: limit function result count.
---

Purpose: limit function result count.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

maximum 3. The rule is part of Strider's extended catalog and runs
when selected with `--only function-result-limit` or when the complete catalog is enabled
with `--all-rules`.
