---
title: weak-cryptography
description: Detect deprecated cryptographic primitives.
---

**Default severity:** 🟡 `warning`

Reports calls into MD5, SHA-1, DES, 3DES, and RC4. These primitives are unsafe
for new security-sensitive designs; checksum-only legacy uses can be excluded
locally through configuration.
