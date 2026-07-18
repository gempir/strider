# Repository instructions

## Required validation

Always run all of the following before completing work:

- `make check`
- `make test`

When you are done with a huge chunk of your work and finishing up run:

- `make corpus-check`

and if that works run 

- `make corpus-update`

## JavaScript tooling

- Use Bun exclusively for JavaScript and TypeScript dependencies and scripts.
- Use `bun install` and `bun run <script>`.
- Do not use npm, pnpm, or Yarn, and do not add their lockfiles.
