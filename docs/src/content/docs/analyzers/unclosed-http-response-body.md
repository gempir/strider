---
title: unclosed-http-response-body
description: Detect locally acquired HTTP response bodies that are not closed.
---

**Default severity:** `error`

A response body retains its connection and related resources until closed.
The check follows local response aliases and accepts an explicit close or an
obvious ownership transfer such as returning the response.
