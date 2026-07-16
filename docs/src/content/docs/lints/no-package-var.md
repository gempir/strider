---
title: no-package-var
description: Avoid mutable variables at package scope.
---

**Default severity:** `warning`  
**Configuration:** `enabled`, `severity`, and path `excludes`; blank identifiers remain exempt

Reports every non-blank name declared by a package-level `var`. Package
variables create shared mutable state and hide dependencies, making tests and
concurrent code harder to reason about.

Local variables and constants are not reported. Package-level blank-identifier
declarations are exempt, which permits compile-time interface assertions.

## Bad

```go
var defaultClient = NewClient()
```

## Good

```go
const defaultLimit = 10

func NewService(client *Client) *Service {
	return &Service{client: client}
}

var _ io.Reader = (*Buffer)(nil)
```

Prefer constants, explicit constructors, and dependency injection. An immutable
value stored in a `var` is still reported because Go does not enforce its
immutability.

## Suppress

```go
//strider:ignore no-package-var
var ErrUnavailable = errors.New("unavailable")
```
