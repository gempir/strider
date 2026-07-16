---
title: defer
description: Detect call and return-value mistakes in defer statements.
---

Purpose: detect defer expressions whose evaluation or return values do not
behave as intended.

## Behavior

The rule reports `defer recover()` (which evaluates `recover` immediately) and
deferred function literals whose returned values are discarded. Calls such as
`defer setup()()` are accepted; the first call runs immediately and the
returned cleanup function is deferred. Loop placement is owned by the
dedicated [`no-defer-in-loop`](../no-defer-in-loop/) rule.

## Default

all checks enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only defer` or when the complete catalog is enabled
with `--all-rules`.
