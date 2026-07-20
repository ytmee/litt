# Bubble Tea v2 for TUI framework

The TUI needs an interactive split-panel browser with keyboard navigation, scrollable views, and text input for search. Bubble Tea v2 provides an Elm-architecture (Model-Update-View) with composable components via Bubbles and styled rendering via Lip Gloss, matching the TUI's requirements closely. The alternative (tview) has a widget-tree model that would fight the split-panel + DAG rendering layout. Termui is unmaintained. Raw tcell would require building every primitive from scratch. The composable, message-passing model of Bubble Tea aligns well with the issue browser's state-heavy interactions (filter, search, tree expand/collapse, view switching).

**Architecture**: Single model delegating to per-view sub-models, communicating via Bubble Tea messages.
**Interface**: The TUI is read-only (view, browse, filter). Write operations are out of scope and handled by CLI/MCP.
**Navigation**: Single-focus master-detail — left panel = filterable issue list, right panel = issue detail on selection. No Tab focus switching between panels. Vim-style `j`/`k` navigation, `/` for title search.
