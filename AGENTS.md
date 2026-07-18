# Repository instructions

Our own codebase must pass `strider check` without any errors or warnings.

## Required validation

Always run all of the following before completing work:

- `make check`
- `make test`
- `make corpus-check`

## JavaScript tooling

- Use Bun exclusively for JavaScript and TypeScript dependencies and scripts.
- Use `bun install` and `bun run <script>`.
- Do not use npm, pnpm, or Yarn, and do not add their lockfiles.
