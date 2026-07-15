---
title: Strider
description: A strict formatter and syntax linter for Go.
template: splash
hero:
  tagline: One fast, self-contained tool for consistent and understandable Go code.
  image:
    file: ../../assets/strider.png
    alt: Strider
  actions:
    - text: Get started
      link: /getting-started/
      icon: right-arrow
    - text: Explore lint rules
      link: /rules/
      variant: minimal
---

Strider combines a width-aware formatter and an AST-only linter in one Go
binary. Its workflows are familiar to users of standard Go tools while its
style and rules are intentionally stricter.

## Current scope

- Deterministic formatting with a strict 100-column profile.
- Seven clarity and safety lint rules.
- Text and JSON diagnostics.
- Recursive source discovery with generated and vendored code excluded.
- Safe formatting guarded by reparse, syntax-tree, comment, and idempotence
  checks.

Strider is an early draft. The command-line and configuration contracts can
still evolve before v1.
