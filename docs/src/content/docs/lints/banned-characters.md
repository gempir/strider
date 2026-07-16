---
title: banned-characters
description: reject configured characters in identifiers.
---

Purpose: reject configured characters in identifiers.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

no banned characters. The rule is part of Strider's extended catalog and runs
when selected with `--only banned-characters` or when the complete catalog is enabled
with `--all-rules`.
