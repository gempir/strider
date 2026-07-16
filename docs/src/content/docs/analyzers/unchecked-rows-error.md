---
title: unchecked-rows-error
description: Detect sql.Rows iteration without an Err check.
---

**Default severity:** `warning`

`Rows.Next` returns false both when iteration finishes successfully and when an
iteration error occurs. Check `Rows.Err` after the loop so driver, network, and
decoding failures are not silently treated as successful completion.

```go
for rows.Next() {
	// scan values
}
// reported: rows.Err is never checked

for rows.Next() {
	// scan values
}
if err := rows.Err(); err != nil { // accepted
	return err
}
```
