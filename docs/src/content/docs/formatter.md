---
title: Formatter
description: Strider's strict formatting profile, write workflows, and safety model.
sidebar:
  badge:
    text: warning
    class: severity-indicator severity-warning
---

**Default severity:** <span class="severity-indicator severity-warning" aria-hidden="true"></span> `warning`

Gofmt is not strict enough in my opinion, the goal of strider's formatter is decide almost every choice of formatting for you.
No discussion in the team about it, just accept the formatting. 
Goal is to be compatible with gofmt as much as possible.

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

The resulting `format` diagnostics use the same text, JSON, and HTML reporters
as every other check.

## Style

Strider uses tabs for indentation, LF line endings, one final newline, and a
180-column print width by default. Imports are sorted into standard-library,
third-party, and current-module groups. Lists that break across lines use one
item per line and a trailing comma.

Function signatures, calls, composite literals, and expressions use the same
bounded group-fitting algorithm. Binary operators remain on the preceding line
so automatic semicolon insertion cannot change the program.

Configure the wrap target and excluded filesystem paths in `strider.toml`:

```toml
version = 1

[formatter]
print-width = 120
excludes = ["internal/generated/**"]
```

Formatter exclusions apply both to `strider fmt` and to the `format` check. See
[Configuration](/configuration/#formatter) for ranges and the fixed parts of
Strider's formatting profile.

## Safety checks

Before a file can be written, Strider:

1. Parses and preflights the complete file.
2. Renders and reparses the result.
3. Confirms that the syntax fingerprint is unchanged.
4. Confirms that comment contents and ordering are unchanged.
5. Formats the result again and requires byte-for-byte idempotence.

For a batch write, every file must pass before temporary files are staged and
atomically renamed. One unsupported or invalid file prevents the entire batch
from being written.

## Current syntax boundary

The formatter supports ordinary application code, including generics, type
switches, `select`, channel sends, `goto`, `fallthrough`, and labeled
statements. Some comments embedded deeply inside expressions remain outside the
current syntax boundary. Refusal is an exit-code `2` error and never produces a
partial file.

Use `//strider:format-ignore` anywhere in a file to return that file unchanged.
This is currently a file-level escape hatch; region and next-node formatting
ignores are not implemented.
