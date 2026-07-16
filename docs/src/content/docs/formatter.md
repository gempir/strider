---
title: Formatter
description: Strider's strict formatting profile, workflows, and safety model.
---

The formatter parses Go source into a lossless concrete syntax tree and renders
that tree directly at a configured width (100 columns by default). Because the
CST retains comments, whitespace, separators, and original token spellings, the
formatter does not need a parallel AST or document model.
It is intentionally independent from `gofmt`: output remains valid and
semantically equivalent Go, but byte-for-byte `gofmt` compatibility is not a
goal.

## Workflows

Write all discovered files in place:

```sh
strider fmt [PATH]...
```

Check without writing:

```sh
strider fmt --check [PATH]...
```

Print full-file unified diffs without writing:

```sh
strider fmt --diff [PATH]...
```

Format standard input:

```sh
strider fmt --stdin --stdin-filename main.go < main.go
```

## Style

Strider uses tabs for indentation, LF line endings, one final newline, and a
100-column print width by default. Imports are sorted into standard-library,
third-party, and current-module groups. Lists that break across lines use one
item per line and a trailing comma.

Function signatures, calls, composite literals, and expressions use the same
bounded group-fitting algorithm. Binary operators remain on the preceding
line so automatic semicolon insertion cannot change the program.

Configure the wrap target, visual indentation width, preserved empty-line cap,
line endings, and excluded filesystem paths in `strider.toml`:

```toml
[formatter]
print-width = 120
indent-width = 4
max-empty-lines = 1
end-of-line = "lf"
excludes = ["internal/generated/**"]
```

See [Configuration](/configuration/#formatter) for ranges and the fixed parts
of Strider's formatting profile.

## Safety checks

Before a file can be written, Strider:

1. Parses and preflights the complete file.
2. Renders and reparses the result.
3. Confirms that the syntax-tree fingerprint is unchanged.
4. Confirms that comment contents and ordering are unchanged.
5. Formats the result again and requires byte-for-byte idempotence.

For a batch write, every file must pass before temporary files are staged and
atomically renamed. One unsupported or invalid file prevents the entire batch
from being written.

## Current syntax boundary

The formatter supports ordinary application code, including generics, type
switches, `select`, channel sends, and labeled statements. It currently refuses
`goto`, `fallthrough`, and some comments embedded deeply inside expressions.
Refusal is an exit-code `2` error and never produces a partial file.

Use `//strider:format-ignore` anywhere in a file to return that file unchanged.
This is currently a file-level escape hatch; region and next-node formatting
ignores are not implemented.
