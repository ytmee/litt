# litt — Local-first task graph and execution tracker for AI agents

## Domain

- **Issue**: A unit of work. The core entity.
  - `state: open | closed` — native lifecycle field.
  - `kind: spec | task | bug` — structural kind.
  - `parent_issue_id: INTEGER NULL` — optional parent issue. Analogous to GitHub's parent/child.
  - Labels: triage, category, or custom — unified in a labels table.
- **Spec**: An issue with `kind=spec`.
- **Bug**: An issue with `kind=bug`.
- **Label**: A tag on an issue.
  - `triage`: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`
  - `category`: `enhancement`
  - `custom`: user-defined freeform labels
  - `kind` metadata triage/category/custom is advisory, not structural.
- **Blocking edge**: A directed dependency from one issue to another, stored in `issue_blocks(blocker_issue_id, blocked_issue_id)`. The graph is a DAG.
- **Blocked**: An issue with at least one incoming blocking edge from an **open** issue. Closed blockers do not count.
- **Ready**: A derived query: `state=open` + label `ready-for-agent` + no incoming `issue_blocks` from open issues.
- **MCP**: Model Context Protocol — the interface AI agents use to interact with litt.
- **CLI**: The command-line interface humans use.
