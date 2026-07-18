---
title: unsafe-formatted-url-host-port
description: Detect URL and network-address construction that breaks IPv6.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Formatting a URL authority or an address passed directly to a standard-library
network listener as `host:port` does not add the brackets required around IPv6
literals. `net.JoinHostPort` correctly handles hostnames, IPv4, and IPv6.

```go
url := fmt.Sprintf("https://%s:%d/path", host, port) // reported

address := net.JoinHostPort(host, strconv.Itoa(port))
url := "https://" + address + "/path" // accepted
```
