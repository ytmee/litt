# AGENTS.md

Drop-in operating instructions for coding agents. Read this file before every task.

**Working code only. Finish the job. Plausibility is not correctness.**

This file follows the [AGENTS.md](https://agents.md) open standard (Linux Foundation / Agentic AI Foundation).

## Project context

### Stack
- Language: Go 1.26
- Module: `github.com/ytmee/litt`
- Package manager: Go modules

### Commands
- Build: `go build ./...`
- Test (all): `go test ./...`
- Test (single): `go test -run <TestName> ./...`
- Lint: `golangci-lint run`
- Run: `go run .`

### Layout
- Source lives in: root `*.go` files and subdirectories
- Tests live next to source: `*_test.go`

## Agent skills

### Issue tracker

Issues tracked in GitHub Issues. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five canonical labels. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — root `CONTEXT.md` + `docs/adr/`. See `docs/agents/domain.md`.

### Commands
- Commit: `git commit -m "type(scope): message"` (conventional commits)
