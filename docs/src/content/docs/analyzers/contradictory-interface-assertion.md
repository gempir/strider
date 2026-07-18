---
title: contradictory-interface-assertion
description: Detect interface assertions with conflicting method signatures.
---

**Default severity:** `error`

An assertion from one interface to another can compile even when the two
method sets contain a same-named method with incompatible signatures. No
dynamic type can implement both contracts, so the assertion can never
succeed.

Assertions to concrete types are already checked by the Go compiler. This
check focuses on interface-to-interface assertions.

```go
type source interface { Read() int }
type target interface { Read() string }

value, ok := input.(target) // reported when input has type source
```
