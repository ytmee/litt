# WebUI is a server-rendered, zero-JS interface

litt is local-first and offline-capable, so the WebUI (`litt web`) is built as a single-serving HTTP server that renders `html/template` pages with Pico CSS vendored and embedded into the binary — no CDN, no JS, no build step. This keeps `go install` / offline environments self-sufficient and matches the read-dominant purpose (CLI and MCP stay the full write surface).

Status: accepted

## Considered options

- **SPA (React/shadcn, Vite build + embed)**: rejected — adds a Node toolchain and a network/CDN dependency, contradicts local-first and offline self-sufficiency. GitHub-grade looks were not worth the build pipeline.
- **HTMX progressive enhancement on server rendering**: deferred — all five write operations are low-frequency forms; full-page POST + 302 is acceptable. A single `htmx.min.js` file can be dropped in later if friction is reported.
- **BeerCSS (Material 3)**: rejected — requires its JS runtime for components (violates zero-JS), is 28% heavier (106 KB vs 83 KB), and its Material look mismatches the GitHub-neutral aesthetic litt follows.

## Deferred scope

- **Full DAG graph visualization**: deferred until real workflow shows untangling blocking chains is dominant (not text reading), or graph exceeds ~hundreds of issues, or multi-context aggregation appears. Surface local blocking edges (one level, closed edges greyed but retained) in the issue sidebar instead.
- **W6 field editing** (title/body/kind/parent): deferred — `UpdateIssue`'s pointer-fields semantics need dirty-tracking in a web form; CLI/MCP already cover it.

## Consequences

- Body/comments render Markdown via goldmark, sanitized by bluemonday, injected as `template.HTML` — the sanitizer is load-bearing and must not be removed.
- Default listener is `127.0.0.1:57664` (fixed port, WSL2 localhost forwarding reaches it from the host browser; bind conflict errors out rather than silently drifting). Non-loopback binds require host-guard allowlist / env opt-in.