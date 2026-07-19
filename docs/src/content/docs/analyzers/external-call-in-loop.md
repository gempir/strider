---
title: external-call-in-loop
description: Detect synchronous SQL and HTTP calls inside loops.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Database queries and HTTP requests issued once per loop iteration commonly
create serial N+1 round trips. The SSA check is limited to known
`database/sql` and `net/http` calls in actual control-flow cycles.

## Bad

```go
for _, id := range ids {
	row := db.QueryRowContext(ctx, query, id)
	_ = row
}
```

## Good

```go
rows, err := db.QueryContext(ctx, batchQuery, ids)
if err != nil {
	return err
}
defer rows.Close()
for rows.Next() {
	mapResult(rows)
}
```
