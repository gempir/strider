---
title: overlapping-encode-buffers
description: Detect overlapping source and destination encoding buffers.
---

**Default severity:** `warning`

Byte encoders that expand their input can overwrite source bytes before
reading them when destination and source begin at the same memory. Use
separate storage or a destination region proven not to overlap.

The analyzer covers the standard ASCII85, base32, base64, and hexadecimal
encoders.

```go
hex.Encode(buffer, buffer)      // reported
hex.Encode(destination, source) // accepted
```
