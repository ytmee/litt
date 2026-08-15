package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// csrfToken reads the synchronizer token from the detail page so write tests
// can replay it in their POST bodies.
func csrfToken(t *testing.T, ts *httptest.Server, issueID int) string {
	t.Helper()
	page := body(t, get(t, ts, fmt.Sprintf("/issues/%d", issueID)))
	const marker = `name="csrf" value="`
	start := strings.Index(page, marker)
	if start == -1 {
		t.Fatal("csrf hidden field missing from detail page")
	}
	start += len(marker)
	end := strings.Index(page[start:], `"`)
	if end == -1 {
		t.Fatal("unterminated csrf value")
	}
	return page[start : start+end]
}

// post sends a form POST without following redirects, so tests can assert the
// 303 and its Location header directly.
func post(t *testing.T, ts *httptest.Server, path string, form url.Values) *http.Response {
	t.Helper()
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.PostForm(ts.URL+path, form)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertRedirect(t *testing.T, resp *http.Response, to string) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != to {
		t.Fatalf("expected redirect to %s, got %q", to, loc)
	}
}

func asideHTML(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, `class="detail-sidebar"`)
	if start == -1 {
		t.Fatal("detail sidebar missing from page")
	}
	rest := page[start:]
	end := strings.Index(rest, "</aside>")
	if end == -1 {
		t.Fatal("detail sidebar has no closing </aside>")
	}
	return rest[:end]
}

// edgeItems extracts each sidebar blocking-edge <li> block, opening tag
// included, so tests can inspect its greyed state.
func edgeItems(section string) []string {
	var out []string
	rest := section
	for {
		start := strings.Index(rest, "<li")
		if start == -1 {
			return out
		}
		end := strings.Index(rest[start:], "</li>")
		if end == -1 {
			return out
		}
		out = append(out, rest[start:start+end+len("</li>")])
		rest = rest[start+end+len("</li>"):]
	}
}

func itemFor(items []string, title string) string {
	for _, it := range items {
		if strings.Contains(it, title) {
			return it
		}
	}
	return ""
}

func TestDetail_RendersMetadataAndBody(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Render check", "spec", "the body text", []string{"enhancement"}); err != nil {
		t.Fatal(err)
	}

	page := body(t, get(t, ts, "/issues/1"))
	for _, want := range []string{">#1<", "Render check", ">spec<", "the body text", "--label-color:#a2eeef\">enhancement"} {
		if !strings.Contains(page, want) {
			t.Errorf("detail page missing %q", want)
		}
	}
}

func TestDetail_RendersBodyMarkdownSanitised(t *testing.T) {
	ts, s := newTestServer(t)
	src := "# Heading\n\nSome **bold** text.\n\n<script>alert('xss')</script>"
	if _, err := s.CreateIssue("Markdown", "spec", src, nil); err != nil {
		t.Fatal(err)
	}

	page := body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, "<h1>Heading</h1>") {
		t.Error("body markdown should render an h1")
	}
	if !strings.Contains(page, "<strong>bold</strong>") {
		t.Error("body markdown should render emphasis")
	}
	if strings.Contains(page, "<script") {
		t.Error("injected script tag must be sanitised out of the body")
	}
}

func TestDetail_RendersCommentsMarkdownSanitised(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Comments", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddComment(1, "First **comment**."); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddComment(1, "<script>alert('xss')</script>"); err != nil {
		t.Fatal(err)
	}

	page := body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, "<strong>comment</strong>") {
		t.Error("comment markdown should render emphasis")
	}
	if strings.Contains(page, "<script") {
		t.Error("injected script tag must be sanitised out of comments")
	}
}

func TestDetail_SidebarListsEdgesGreyedClosed(t *testing.T) {
	ts, s := newTestServer(t)
	// 1 blocked by 2 (open) and 3 (closed); 1 blocks 4
	for _, tc := range []struct {
		title string
		kind  string
	}{
		{"Main issue", "task"},
		{"Open blocker", "spec"},
		{"Closed blocker", "bug"},
		{"Blocked child", "task"},
	} {
		if _, err := s.CreateIssue(tc.title, tc.kind, "", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateBlock(2, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(1, 4); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(3); err != nil {
		t.Fatal(err)
	}

	items := edgeItems(asideHTML(t, body(t, get(t, ts, "/issues/1"))))

	open := itemFor(items, "Open blocker")
	if open == "" || strings.Contains(open, "edge-closed") {
		t.Error("open blocker should be listed and not greyed")
	}
	closed := itemFor(items, "Closed blocker")
	if closed == "" || !strings.Contains(closed, "edge-closed") {
		t.Error("closed blocker should be listed but greyed")
	}
	if itemFor(items, "Blocked child") == "" {
		t.Error("blocked issue should appear in the Blocks list")
	}
}

func TestDetail_SidebarLinksNavigate(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main issue", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("Blocking issue", "spec", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(2, 1); err != nil {
		t.Fatal(err)
	}

	page := body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, `href="/issues/2">#2 Blocking issue</a>`) {
		t.Fatal("blocking edge should link to the related issue")
	}
	linked := body(t, get(t, ts, "/issues/2"))
	if !strings.Contains(linked, "Blocking issue") {
		t.Fatal("linked issue page should load")
	}
}

func TestDetail_CloseReopenRedirectsAndFlipsState(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Toggle me", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	assertRedirect(t, post(t, ts, "/issues/1/state", url.Values{"csrf": {csrf}, "state": {"closed"}}), "/issues/1")
	page := body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, ">closed<") {
		t.Fatal("issue should render as closed after close POST")
	}
	if !strings.Contains(page, "Reopen issue") {
		t.Fatal("closed issue should offer a reopen action")
	}

	assertRedirect(t, post(t, ts, "/issues/1/state", url.Values{"csrf": {csrf}, "state": {"open"}}), "/issues/1")
	page = body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, "Close issue") {
		t.Fatal("open issue should offer a close action")
	}
	if strings.Contains(page, ">closed<") {
		t.Fatal("issue should render as open after reopen POST")
	}
}

func TestDetail_AddRemoveLabelRedirectAndEffect(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Labelled", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	assertRedirect(t, post(t, ts, "/issues/1/labels", url.Values{"csrf": {csrf}, "add": {"enhancement"}}), "/issues/1")
	page := body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, `--label-color:#a2eeef">enhancement</span>`) {
		t.Fatal("added label should render as a pill on the detail page")
	}

	assertRedirect(t, post(t, ts, "/issues/1/labels", url.Values{"csrf": {csrf}, "remove": {"enhancement"}}), "/issues/1")
	page = body(t, get(t, ts, "/issues/1"))
	if strings.Contains(page, `--label-color:#a2eeef">enhancement</span>`) {
		t.Fatal("removed label should no longer render as a pill")
	}
}

func TestDetail_AddCommentRedirectAndEffect(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Discussed", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	assertRedirect(t, post(t, ts, "/issues/1/comments", url.Values{"csrf": {csrf}, "body": {"A **new** comment"}}), "/issues/1")
	page := body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, "<strong>new</strong>") {
		t.Fatal("posted comment should appear on the detail page")
	}
	if strings.Contains(page, "No comments yet.") {
		t.Fatal("comment list should not show the empty state after a comment")
	}
}

func TestDetail_EmptyCommentRedirectsWithoutError(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Discussed", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	assertRedirect(t, post(t, ts, "/issues/1/comments", url.Values{"csrf": {csrf}, "body": {""}}), "/issues/1")
	page := body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, "No comments yet.") {
		t.Fatal("empty comment should not be appended")
	}
}

func TestDetail_UnknownIssueReturns404(t *testing.T) {
	ts, _ := newTestServer(t)

	for _, tc := range []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodGet, "/issues/999", nil},
		{http.MethodGet, "/issues/abc", nil},
		{http.MethodPost, "/issues/999/state", url.Values{"state": {"closed"}}},
		{http.MethodPost, "/issues/999/labels", url.Values{"add": {"enhancement"}}},
		{http.MethodPost, "/issues/999/comments", url.Values{"body": {"hi"}}},
	} {
		var resp *http.Response
		if tc.method == http.MethodPost {
			resp = post(t, ts, tc.path, tc.form)
		} else {
			resp = get(t, ts, tc.path)
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s %s: expected 404, got %d", tc.method, tc.path, resp.StatusCode)
			}
		}()
	}
}

func TestDetail_InvalidStateRejected(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("State", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	resp := post(t, ts, "/issues/1/state", url.Values{"csrf": {csrf}, "state": {"bogus"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid state, got %d", resp.StatusCode)
	}
}

func TestDetail_RendersOpenState(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Open issue", "task", "", nil); err != nil {
		t.Fatal(err)
	}

	page := body(t, get(t, ts, "/issues/1"))
	if !strings.Contains(page, "issue-state-open\">open<") {
		t.Fatal("open issue should render an open state badge")
	}
	if strings.Contains(page, "issue-state-closed") {
		t.Fatal("open issue should not render a closed state badge")
	}
}

func TestDetail_WriteRejectedWithoutCSRF(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Guarded", "task", "", nil); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		form url.Values
	}{
		{"/issues/1/state", url.Values{"state": {"closed"}}},
		{"/issues/1/labels", url.Values{"add": {"enhancement"}}},
		{"/issues/1/comments", url.Values{"body": {"hi"}}},
	} {
		resp := post(t, ts, tc.path, tc.form)
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("POST %s without csrf: expected 403, got %d", tc.path, resp.StatusCode)
			}
		}()
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.State != "open" || len(issue.Labels) != 0 {
		t.Fatal("rejected write must not mutate the issue")
	}
	comments, err := s.ListComments(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 0 {
		t.Fatal("rejected comment must not be appended")
	}
}

func TestList_RowsLinkToDetail(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Linked issue", "task", "", nil); err != nil {
		t.Fatal(err)
	}

	page := body(t, get(t, ts, "/"))
	if !strings.Contains(page, `href="/issues/1">Linked issue</a>`) {
		t.Fatal("list row title should link to the issue detail page")
	}
}