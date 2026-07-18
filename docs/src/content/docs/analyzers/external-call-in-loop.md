---
title: external-call-in-loop
description: Detect synchronous SQL and HTTP calls inside loops.
---

**Default severity:** 🟡 `warning`

Database queries and HTTP requests issued once per loop iteration commonly
create serial N+1 round trips. The SSA check is limited to known
`database/sql` and `net/http` calls in actual control-flow cycles.
