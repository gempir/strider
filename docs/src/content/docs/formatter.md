---
title: Formatter
description: Strider's strict formatting profile and write workflows.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Strider applies one deterministic formatting profile so a team does not need to
make recurring layout decisions in review. It follows gofmt-compatible Go
syntax while also making line wrapping, import grouping, and multiline layout
consistent.

## Workflows

Write all discovered files in place:

```sh
strider fmt [PATH]...
```

Print full-file unified diffs without writing:

```sh
strider fmt --diff [PATH]...
```

Format standard input:

```sh
strider fmt --stdin --stdin-filename main.go < main.go
```

For a read-only formatting check in CI, use the unified diagnostic command:

```sh
strider check --only format [PATH]...
```

Apply the same validated formatting candidate through the unified command with:

```sh
strider check --only format --fix [PATH]...
```

The resulting `format` diagnostics use the same text, JSON, and HTML reporters
as every other check. Fix mode composes any granular rule edits first, then
formats each affected source.

## Bad

```go
package main
import("fmt"; "os")
func main(){fmt.Println(os.Args)}
```

## Good

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.Args)
}
```

## Style

Strider uses tabs for indentation, LF line endings, one final newline, and a
180-column print width by default. Imports are sorted into standard-library,
third-party, and current-module groups. Lists that break across lines use one
item per line and a trailing comma.

Function signatures and calls use the same bounded group-fitting algorithm.
Every non-empty composite literal uses one element per line with a trailing
comma, including structs, maps, slices, and arrays. Binary expressions remain
flat; manually broken binary operators remain on the preceding line so
automatic semicolon insertion cannot change the program. Top-level declarations
are ordered as constants, variables, types, then functions while preserving
relative order within each kind.

Configure the wrap target and excluded filesystem paths in `strider.toml`:

```toml
version = 1

[formatter]
print-width = 120
excludes = ["internal/generated/**"]
```

Formatter exclusions apply both to `strider fmt` and to the `format` check. See
[Configuration](/configuration/#formatter) for the width range and Strider's
fixed formatting profile.
