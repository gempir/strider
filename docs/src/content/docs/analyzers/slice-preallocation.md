---
title: slice-preallocation
description: Detect slices that can use range-source capacity.
---

**Default severity:** `warning`

Conservatively reports an empty slice followed by exactly one direct append per
iteration of a range with a useful `len`. Preallocating that capacity avoids
repeated growth while preserving zero length.
