---
title: bidirectional-control-character
description: Reject invisible bidirectional source controls.
---

Purpose: detect invisible Unicode controls that can change the visual order of
source text without changing its logical byte order. Such controls can make
reviewed code appear to mean something different from what the compiler reads.

Strider default: available in the optional check catalog at `error` severity.

The rule covers embedding, override, isolate, and matching pop controls. Write
ordinary direction-neutral source text instead.
