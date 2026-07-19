---
title: overlapping-encode-buffers
description: Detect overlapping source and destination encoding buffers.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Byte encoders that expand their input can overwrite source bytes before
reading them when destination and source begin at the same memory. Use
separate storage or a destination region proven not to overlap.

The check covers the standard ASCII85, base32, base64, and hexadecimal
encoders.

## Bad

```go
hex.Encode(buffer, buffer)
```

## Good

```go
hex.Encode(destination, source)
```
