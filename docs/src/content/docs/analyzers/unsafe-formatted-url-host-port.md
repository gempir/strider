---
title: unsafe-formatted-url-host-port
description: Detect URL host and port construction that breaks IPv6.
---

**Default severity:** `warning`

Formatting a URL authority as `host:port` does not add the brackets required
around IPv6 literals. `net.JoinHostPort` correctly handles hostnames, IPv4, and
IPv6.

```go
url := fmt.Sprintf("https://%s:%d/path", host, port) // reported

address := net.JoinHostPort(host, strconv.Itoa(port))
url := "https://" + address + "/path" // accepted
```
