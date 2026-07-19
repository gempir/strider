---
title: unclosed-http-response-body
description: Detect locally acquired HTTP response bodies that are not closed.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

A response body retains its connection and related resources until closed.
The check follows local response aliases and accepts an explicit close or an
obvious ownership transfer such as returning the response.

## Bad

```go
response, err := http.Get(endpoint)
if err != nil {
	return err
}
return decode(response.Body)
```

## Good

```go
response, err := http.Get(endpoint)
if err != nil {
	return err
}
defer response.Body.Close()
return decode(response.Body)
```
