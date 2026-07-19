## litt — local issue tracker

litt stores features, tasks, labels, and blocking relationships in SQLite.
Use its MCP tools instead of creating Markdown issue files.

### Body format

Write issue bodies in **Markdown**. Both humans (`litt issue show`) and agents
(`get_issue`) read the raw body — Markdown keeps it readable in the terminal
and structured enough for automated parsing. Use headings, lists, checkboxes,
and code blocks to organise content.

### When to use each tool

| When you need | Call this |
|---|---|
| Create a spec, task, or bug | `create_issue(kind="spec"\|"task"\|"bug", title, ...)` |
| Find what to work on next | `get_ready_issues` |
| Mark an issue as done | `update_issue(number, state="closed")` |
| Break work into subtasks | `create_issue` + `set_parent` |
| Express a dependency | `add_blocking(blocker_number, blocked_number)` |
| Read issue details | `get_issue(number)` |
| Search by state/kind/label | `query_issues(state?, kind?, label?, ...)` |
| Create a label | `create_label(name, kind?, description?)` |
| Delete a label | `delete_label(name)` |
| List all labels | `list_labels` |
| Add a comment to an issue | `add_comment(number, body)` |
| List comments on an issue | `get_comments(number)` |
