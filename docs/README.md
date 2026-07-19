# Strider documentation

This directory contains the Strider documentation site, built with
[Starlight](https://starlight.astro.build/). Bun and Go are required: the dev
server and production build regenerate compact lint pages from the Go rule
registry and export the current catalog size before Astro starts. Homepage
performance figures come from the measured corpus snapshot refreshed by
`make corpus-update`.

```sh
bun install
bun run dev
```

Run `bun run build` to create the static site in `dist/`.
