# litt — Local-first task graph and execution tracker for AI agents

## Domain

- **Issue**: A unit of work. The core entity.
  - `state: open | closed` — native lifecycle field.
  - `kind: feature | task` — structural kind. `bug`/`enhancement` are category labels, not kinds.
  - `parent_issue_id: INTEGER NULL` — optional parent issue. Analogous to GitHub's parent/child.
  - Labels: triage, category, or custom — unified in a labels table.
- **Feature**: An issue with `kind=feature`. No separate feature table.
- **Label**: A tag on an issue.
  - `triage`: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`
  - `category`: `bug`, `enhancement`
  - `custom`: user-defined freeform labels
  - `kind` metadata triage/category/custom is advisory, not structural.
- **Blocking edge**: A directed dependency from one issue to another, stored in `issue_blocks(blocker_issue_id, blocked_issue_id)`. The graph is a DAG.
- **Ready**: A derived query: `state=open` + label `ready-for-agent` + no incoming `issue_blocks` from open issues.
- **MCP**: Model Context Protocol — the interface AI agents use to interact with litt.
- **CLI**: The command-line interface humans use.
