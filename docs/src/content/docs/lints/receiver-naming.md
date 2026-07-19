---
title: receiver-naming
description: "Enforce consistent receiver names."
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Rejects receiver names `this` and `self`, and requires every method on the same
receiver type to use the same receiver name. It does not derive or enforce a
particular abbreviation and does not impose a maximum length.

## Bad

```go
func (self *Client) Send() error
```

## Good

```go
func (c *Client) Send() error
```
