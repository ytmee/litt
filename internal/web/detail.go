package web

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/ytmee/litt/internal/store"
)

type commentView struct {
	ID        int
	BodyHTML  template.HTML
	CreatedAt string
}

// edgeList carries a sidebar blocking-edge list and its empty-state message.
type edgeList struct {
	Items        []store.Issue
	Empty        string
	RemoveAction string
	CSRF         string
}

type detailPageData struct {
	Title         string
	CSRF          string
	Issue         store.Issue
	BodyHTML      template.HTML
	Comments      []commentView
	BlockedBy     edgeList
	Blocking      edgeList
	AddableLabels []store.Label
	Error         string
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIssueID(w, r)
	if !ok {
		return
	}
	h.renderDetail(w, r, id, "")
}

// renderDetail loads the detail-page data and renders it. It is shared by the
// GET handler and the edge write handlers so a rejected write surfaces its
// reason on the same page.
func (h *Handler) renderDetail(w http.ResponseWriter, r *http.Request, id int, errMsg string) {
	issue, err := h.store.GetIssue(id)
	if err != nil {
		h.writeIssueError(w, r, err)
		return
	}
	comments, err := h.store.ListComments(id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	blockedBy, err := h.store.ListBlockedBy(id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	blocking, err := h.store.ListBlocking(id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	allLabels, err := h.store.ListLabels()
	if err != nil {
		h.serverError(w, err)
		return
	}

	commentViews := make([]commentView, 0, len(comments))
	for _, c := range comments {
		commentViews = append(commentViews, commentView{
			ID:        c.ID,
			BodyHTML:  renderMarkdown(c.Body),
			CreatedAt: c.CreatedAt,
		})
	}

	data := detailPageData{
		Title:         fmt.Sprintf("#%d %s — litt", issue.ID, issue.Title),
		CSRF:          h.csrf,
		Issue:         *issue,
		BodyHTML:      renderMarkdown(issue.Body),
		Comments:      commentViews,
		BlockedBy:     edgeList{Items: blockedBy, Empty: "Nothing blocks this issue.", RemoveAction: fmt.Sprintf("/issues/%d/blocked-by/remove", id), CSRF: h.csrf},
		Blocking:      edgeList{Items: blocking, Empty: "Blocks nothing.", RemoveAction: fmt.Sprintf("/issues/%d/blocking/remove", id), CSRF: h.csrf},
		AddableLabels: addableLabels(allLabels, issue.Labels),
		Error:         errMsg,
	}

	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, "detail.html", data); err != nil {
		h.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if errMsg != "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	_, _ = w.Write(buf.Bytes())
}

// addBlocking records "this issue blocks target", then redirects back.
func (h *Handler) addBlocking(w http.ResponseWriter, r *http.Request) {
	h.addEdge(w, r, true)
}

// addBlockedBy records "target blocks this issue", then redirects back.
func (h *Handler) addBlockedBy(w http.ResponseWriter, r *http.Request) {
	h.addEdge(w, r, false)
}

// removeBlocking deletes "this issue blocks target", then redirects back.
func (h *Handler) removeBlocking(w http.ResponseWriter, r *http.Request) {
	h.removeEdge(w, r, true)
}

// removeBlockedBy deletes "target blocks this issue", then redirects back.
func (h *Handler) removeBlockedBy(w http.ResponseWriter, r *http.Request) {
	h.removeEdge(w, r, false)
}

// addEdge adds a blocking edge between the current issue and target. With
// forward=true the current issue blocks target; otherwise target blocks it.
func (h *Handler) addEdge(w http.ResponseWriter, r *http.Request, forward bool) {
	id, ok := h.requireIssue(w, r)
	if !ok {
		return
	}
	if !h.checkCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	target, msg, err := h.parseEdgeTarget(w, r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if msg != "" {
		h.renderDetail(w, r, id, msg)
		return
	}
	if target == id {
		h.renderDetail(w, r, id, "An issue cannot block or be blocked by itself.")
		return
	}
	blocker, blocked := id, target
	if !forward {
		blocker, blocked = target, id
	}
	created, err := h.store.CreateBlock(blocker, blocked)
	if err != nil {
		if errors.Is(err, store.ErrBlockCycle) {
			h.renderDetail(w, r, id, "Adding this edge would create a cycle.")
			return
		}
		h.serverError(w, err)
		return
	}
	if !created {
		if forward {
			msg = fmt.Sprintf("Issue already blocks #%d.", target)
		} else {
			msg = fmt.Sprintf("Issue already blocked by #%d.", target)
		}
		h.renderDetail(w, r, id, msg)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/issues/%d", id), http.StatusSeeOther)
}

// removeEdge removes the blocking edge between the current issue and target.
// With forward=true the current issue blocks target; otherwise target blocks it.
func (h *Handler) removeEdge(w http.ResponseWriter, r *http.Request, forward bool) {
	id, ok := h.requireIssue(w, r)
	if !ok {
		return
	}
	if !h.checkCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	target, msg, err := h.parseEdgeTarget(w, r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if msg != "" {
		h.renderDetail(w, r, id, msg)
		return
	}
	blocker, blocked := id, target
	if !forward {
		blocker, blocked = target, id
	}
	if err := h.store.RemoveBlock(blocker, blocked); err != nil {
		if errors.Is(err, store.ErrBlockNotFound) {
			h.renderDetail(w, r, id, "Block edge not found.")
			return
		}
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/issues/%d", id), http.StatusSeeOther)
}

// parseEdgeTarget parses and verifies the target issue for an edge write,
// returning an error message when the target is unusable.
func (h *Handler) parseEdgeTarget(w http.ResponseWriter, r *http.Request) (int, string, error) {
	target, err := strconv.Atoi(r.FormValue("target"))
	if err != nil || target < 1 {
		return 0, "Target must be a positive issue number.", nil
	}
	if _, err := h.store.GetIssue(target); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return 0, fmt.Sprintf("Issue #%d not found.", target), nil
		}
		return 0, "", err
	}
	return target, "", nil
}

// setState closes or reopens an issue via a form POST, then redirects back.
func (h *Handler) setState(w http.ResponseWriter, r *http.Request) {
	id, ok := h.requireIssue(w, r)
	if !ok {
		return
	}
	if !h.checkCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	var err error
	switch r.FormValue("state") {
	case "open":
		err = h.store.ReopenIssue(id)
	case "closed":
		err = h.store.CloseIssue(id)
	default:
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/issues/%d", id), http.StatusSeeOther)
}

// setLabels adds or removes a single label via a form POST, then redirects back.
func (h *Handler) setLabels(w http.ResponseWriter, r *http.Request) {
	id, ok := h.requireIssue(w, r)
	if !ok {
		return
	}
	if !h.checkCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	var add, remove []string
	if name := r.FormValue("add"); name != "" {
		add = []string{name}
	}
	if name := r.FormValue("remove"); name != "" {
		remove = []string{name}
	}
	if len(add) > 0 || len(remove) > 0 {
		if err := h.store.UpdateIssueLabels(id, add, remove); err != nil {
			h.serverError(w, err)
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/issues/%d", id), http.StatusSeeOther)
}

// addComment appends a comment via a form POST, then redirects back.
func (h *Handler) addComment(w http.ResponseWriter, r *http.Request) {
	id, ok := h.requireIssue(w, r)
	if !ok {
		return
	}
	if !h.checkCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	if body := r.FormValue("body"); body != "" {
		if _, err := h.store.AddComment(id, body); err != nil {
			h.serverError(w, err)
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/issues/%d", id), http.StatusSeeOther)
}

// writeIssueError maps an unknown issue to 404 and any real store failure to 500.
func (h *Handler) writeIssueError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	h.serverError(w, err)
}

// parseIssueID resolves the {id} path value to a positive integer, returning
// 404 for malformed ids.
func parseIssueID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

// requireIssue resolves a valid issue id and verifies the issue exists,
// returning 404 otherwise.
func (h *Handler) requireIssue(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, ok := parseIssueID(w, r)
	if !ok {
		return 0, false
	}
	if _, err := h.store.GetIssue(id); err != nil {
		h.writeIssueError(w, r, err)
		return 0, false
	}
	return id, true
}

// checkCSRF verifies the single synchronizer token embedded in every write form.
func (h *Handler) checkCSRF(r *http.Request) bool {
	return subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(h.csrf)) == 1
}

func newCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("web: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func addableLabels(all, current []store.Label) []store.Label {
	have := make(map[int]bool, len(current))
	for _, l := range current {
		have[l.ID] = true
	}
	out := make([]store.Label, 0, len(all))
	for _, l := range all {
		if !have[l.ID] {
			out = append(out, l)
		}
	}
	return out
}
