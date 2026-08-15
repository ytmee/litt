package web

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/ytmee/litt/internal/store"
)

type newPageData struct {
	Title    string
	Body     string
	Kind     string
	Labels   []store.Label
	Checked  map[string]bool
	Selected []string
	Error    string
	CSRF     string
}

// newForm renders the create-issue form.
func (h *Handler) newForm(w http.ResponseWriter, r *http.Request) {
	h.renderNew(w, r, newPageData{CSRF: h.csrf})
}

// createIssue validates the submitted fields and creates the issue, re-rendering
// the form with the reason when validation fails.
func (h *Handler) createIssue(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	title := r.FormValue("title")
	kind := r.FormValue("kind")
	body := r.FormValue("body")
	labels := r.Form["labels"]

	data := newPageData{
		Title:    title,
		Body:     body,
		Kind:     kind,
		Selected: labels,
		CSRF:     h.csrf,
	}

	switch {
	case strings.TrimSpace(title) == "":
		data.Error = "Title is required."
	case !store.ValidKinds[kind]:
		data.Error = fmt.Sprintf("Invalid kind %q: must be one of spec, task, or bug.", kind)
	default:
		msg, err := h.checkLabels(labels)
		if err != nil {
			h.serverError(w, err)
			return
		}
		data.Error = msg
	}

	if data.Error != "" {
		h.renderNew(w, r, data)
		return
	}

	issue, err := h.store.CreateIssue(title, kind, body, labels)
	if err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/issues/%d", issue.ID), http.StatusSeeOther)
}

// checkLabels verifies every submitted label exists, returning a rejection
// message or "".
func (h *Handler) checkLabels(labels []string) (string, error) {
	if len(labels) == 0 {
		return "", nil
	}
	all, err := h.store.ListLabels()
	if err != nil {
		return "", err
	}
	valid := make(map[string]bool, len(all))
	for _, l := range all {
		valid[l.Name] = true
	}
	for _, name := range labels {
		if !valid[name] {
			return fmt.Sprintf("Label %q does not exist.", name), nil
		}
	}
	return "", nil
}

// renderNew renders the create form, re-populating submitted values on errors.
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, data newPageData) {
	if data.Labels == nil {
		labels, err := h.store.ListLabels()
		if err != nil {
			h.serverError(w, err)
			return
		}
		data.Labels = labels
	}
	checked := make(map[string]bool, len(data.Selected))
	for _, name := range data.Selected {
		checked[name] = true
	}
	data.Checked = checked
	data.CSRF = h.csrf

	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, "new.html", data); err != nil {
		h.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data.Error != "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	_, _ = w.Write(buf.Bytes())
}
