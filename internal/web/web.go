package web

import (
	"bytes"
	"embed"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ytmee/litt/internal/query"
	"github.com/ytmee/litt/internal/store"
)

//go:embed assets/* templates/*.html
var content embed.FS

// Options configures the WebUI handler.
type Options struct {
	// BindHost is the concrete host the server binds to; when non-wildcard it
	// is added to the Host-guard allowlist. Wildcard binds expose only loopback
	// hosts unless AllowExternal is set.
	BindHost string
	// AllowExternal disables the Host guard, acknowledging deliberate exposure
	// of the UI beyond loopback.
	AllowExternal bool
}

// Handler serves the litt WebUI.
type Handler struct {
	store *store.Store
	tmpl  *template.Template
	csrf  string
}

// NewHandler returns the WebUI HTTP handler with the host guard and security
// headers applied.
func NewHandler(st *store.Store, opts Options) http.Handler {
	h := &Handler{
		store: st,
		tmpl:  template.Must(template.ParseFS(content, "templates/*.html")),
		csrf:  newCSRFToken(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.list)
	mux.HandleFunc("GET /issue/new", h.newForm)
	mux.HandleFunc("POST /issue/new", h.createIssue)
	mux.HandleFunc("GET /issues/{id}", h.detail)
	mux.HandleFunc("POST /issues/{id}/state", h.setState)
	mux.HandleFunc("POST /issues/{id}/labels", h.setLabels)
	mux.HandleFunc("POST /issues/{id}/comments", h.addComment)
	mux.HandleFunc("POST /issues/{id}/blocking", h.addBlocking)
	mux.HandleFunc("POST /issues/{id}/blocked-by", h.addBlockedBy)
	mux.HandleFunc("POST /issues/{id}/blocking/remove", h.removeBlocking)
	mux.HandleFunc("POST /issues/{id}/blocked-by/remove", h.removeBlockedBy)
	mux.HandleFunc("GET /static/", h.static)

	var handler http.Handler = mux
	if !opts.AllowExternal {
		handler = hostGuard(bindHostAllowlist(opts.BindHost))(handler)
	}
	// Security headers must be outermost so every response — including 403s
	// from the host guard — carries them.
	return securityHeaders(handler)
}

type tab struct {
	Name   string
	Title  string
	Active bool
}

type listRow struct {
	store.Issue
	Blocked bool
}

type listPageData struct {
	Title  string
	View   string
	Kind   string
	Label  string
	Tabs   []tab
	Labels []store.Label
	Rows   []listRow
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	view := parseView(q.Get("view"))
	kind := q.Get("kind")
	label := q.Get("label")

	issues, err := h.issuesFor(view, kind, label)
	if err != nil {
		h.serverError(w, err)
		return
	}

	blocked, err := h.blockedSet(view, kind, label)
	if err != nil {
		h.serverError(w, err)
		return
	}

	rows := make([]listRow, 0, len(issues))
	for _, iss := range issues {
		rows = append(rows, listRow{Issue: iss, Blocked: view == viewBlocked || blocked[iss.ID]})
	}

	labels, err := h.store.ListLabels()
	if err != nil {
		h.serverError(w, err)
		return
	}

	data := listPageData{
		Title:  "litt — task graph",
		View:   view.name(),
		Kind:   kind,
		Label:  label,
		Tabs:   tabs(view),
		Labels: labels,
		Rows:   rows,
	}

	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, "list.html", data); err != nil {
		h.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) issuesFor(v view, kind, label string) ([]store.Issue, error) {
	switch v {
	case viewReady:
		issues, err := query.ListReady(h.store)
		if err != nil {
			return nil, err
		}
		return filterBy(issues, kind, label), nil
	case viewBlocked:
		blocked := true
		return query.ListIssues(h.store, query.Params{State: "open", Kind: kind, Label: label, IsBlocked: &blocked})
	case viewAll:
		return query.ListIssues(h.store, query.Params{Kind: kind, Label: label})
	default:
		return query.ListIssues(h.store, query.Params{State: "open", Kind: kind, Label: label})
	}
}

func (h *Handler) blockedSet(v view, kind, label string) (map[int]bool, error) {
	set := make(map[int]bool)
	if v == viewBlocked || v == viewReady {
		return set, nil
	}
	blocked := true
	params := query.Params{Kind: kind, Label: label, IsBlocked: &blocked}
	if v != viewAll {
		params.State = "open"
	}
	issues, err := query.ListIssues(h.store, params)
	if err != nil {
		return nil, err
	}
	for _, iss := range issues {
		set[iss.ID] = true
	}
	return set, nil
}

func (h *Handler) static(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	data, err := content.ReadFile("assets/" + path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(path))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

func (h *Handler) serverError(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

type view int

const (
	viewOpen view = iota
	viewReady
	viewBlocked
	viewAll
)

func parseView(s string) view {
	switch s {
	case "ready":
		return viewReady
	case "blocked":
		return viewBlocked
	case "all":
		return viewAll
	default:
		return viewOpen
	}
}

func (v view) name() string {
	switch v {
	case viewReady:
		return "ready"
	case viewBlocked:
		return "blocked"
	case viewAll:
		return "all"
	default:
		return "open"
	}
}

func tabs(active view) []tab {
	all := []struct {
		name  string
		title string
	}{
		{viewOpen.name(), "Open"},
		{viewReady.name(), "Ready"},
		{viewBlocked.name(), "Blocked"},
		{viewAll.name(), "All"},
	}
	out := make([]tab, 0, len(all))
	for _, t := range all {
		out = append(out, tab{Name: t.name, Title: t.title, Active: t.name == active.name()})
	}
	return out
}

func filterBy(issues []store.Issue, kind, label string) []store.Issue {
	out := make([]store.Issue, 0, len(issues))
	for _, iss := range issues {
		if kind != "" && iss.Kind != kind {
			continue
		}
		if label != "" && !hasLabel(iss, label) {
			continue
		}
		out = append(out, iss)
	}
	return out
}

func hasLabel(iss store.Issue, name string) bool {
	for _, l := range iss.Labels {
		if l.Name == name {
			return true
		}
	}
	return false
}
