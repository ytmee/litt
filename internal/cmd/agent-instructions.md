## litt — local issue tracker

litt stores features, tasks, labels, and blocking relationships in SQLite.
Use its MCP tools instead of creating Markdown issue files.

### When to use each tool

| When you need | Call this |
|---|---|
| Propose a new feature or task | `create_issue(kind="feature"\|"task", title, ...)` |
| Find what to work on next | `get_ready_issues` |
| Mark an issue as done | `update_issue(number, state="closed")` |
| Break work into subtasks | `create_issue` + `set_parent` |
| Express a dependency | `add_blocking(blocker_number, blocked_number)` |
| Read issue details | `get_issue(number)` |
| Search by state/kind/label | `query_issues(state?, kind?, label?, ...)` |
