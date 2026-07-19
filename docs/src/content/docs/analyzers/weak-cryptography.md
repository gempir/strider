---
title: weak-cryptography
description: Detect deprecated cryptographic primitives.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Reports calls into MD5, SHA-1, DES, 3DES, and RC4. These primitives are unsafe
for new security-sensitive designs; checksum-only legacy uses can be excluded
locally through configuration.

## Bad

```go
sum := md5.Sum(data)
```

## Good

```go
sum := sha256.Sum256(data)
```
