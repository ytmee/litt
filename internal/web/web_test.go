package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ytmee/litt/internal/store"
)

// newTestServer builds a real store over a temp-file DB and serves it through
// an httptest server with the production handler chain.
func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	s, err := store.Ensure(filepath.Join(t.TempDir(), "litt.db"))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewHandler(s, Options{BindHost: "127.0.0.1"}))
	t.Cleanup(func() {
		ts.Close()
		s.Close()
	})
	return ts, s
}

// seedFixtures covers the blocked-badge edge cases: an issue blocked by an open
// issue and one blocked only by a closed issue.
func seedFixtures(t *testing.T, s *store.Store) {
	t.Helper()
	for _, tc := range []struct {
		title  string
		kind   string
		labels []string
	}{
		{"Blocked by open", "task", []string{"needs-triage"}},
		{"Blocked by closed", "task", []string{"enhancement"}},
		{"Open blocker", "spec", nil},
		{"Closed blocker", "bug", nil},
		{"Open unblocked", "task", []string{"enhancement"}},
	} {
		if _, err := s.CreateIssue(tc.title, tc.kind, "", tc.labels); err != nil {
			t.Fatal(err)
		}
	}
	// 1 <- 3 (open blocker), 2 <- 4 (closed blocker)
	if _, err := s.CreateBlock(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(4, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(4); err != nil {
		t.Fatal(err)
	}
}

// csrfFromPage reads the synchronizer token hidden field from any rendered
// page so write tests can replay it in their POST bodies.
func csrfFromPage(t *testing.T, ts *httptest.Server, path string) string {
	t.Helper()
	page := body(t, get(t, ts, path))
	const marker = `name="csrf" value="`
	start := strings.Index(page, marker)
	if start == -1 {
		t.Fatalf("csrf hidden field missing from %s", path)
	}
	start += len(marker)
	end := strings.Index(page[start:], `"`)
	if end == -1 {
		t.Fatal("unterminated csrf value")
	}
	return page[start : start+end]
}

func get(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := ts.Client().Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// rows returns the inner HTML of each issue-row list item, in page order.
func rows(page string) []string {
	var out []string
	marker := `<li class="issue-row">`
	for {
		start := strings.Index(page, marker)
		if start == -1 {
			return out
		}
		rest := page[start+len(marker):]
		end := strings.Index(rest, `</li>`)
		if end == -1 {
			return out
		}
		out = append(out, rest[:end])
		page = page[start+len(marker)+end+len(`</li>`):]
	}
}

func rowContains(page, title string) bool {
	for _, row := range rows(page) {
		if strings.Contains(row, title) {
			return true
		}
	}
	return false
}

func rowBadged(page, title string) bool {
	for _, row := range rows(page) {
		if strings.Contains(row, title) {
			return strings.Contains(row, "blocked-badge")
		}
	}
	return false
}

func TestList_DefaultOpenView(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	resp := get(t, ts, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	page := body(t, resp)

	if !strings.Contains(page, "aria-current=\"page\">Open") {
		t.Fatal("Open tab should be active by default")
	}
	for _, want := range []string{">Open<", ">Ready<", ">Blocked<", ">All<"} {
		if !strings.Contains(page, want) {
			t.Errorf("missing tab %s", want)
		}
	}
	for _, want := range []string{"Blocked by open", "Open blocker", "Open unblocked"} {
		if !strings.Contains(page, want) {
			t.Errorf("expected open issue %q in default list", want)
		}
	}
	if strings.Contains(page, "Closed blocker") {
		t.Fatal("closed issue should not appear in default (open) view")
	}
}

func TestList_UnknownViewDefaultsToOpen(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	page := body(t, get(t, ts, "/?view=bogus"))
	if !strings.Contains(page, "aria-current=\"page\">Open") {
		t.Fatal("unknown view should fall back to Open tab")
	}
	if strings.Contains(page, "Closed blocker") {
		t.Fatal("unknown view should behave like Open and hide closed issues")
	}
}

func TestList_AllViewIncludesClosed(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	page := body(t, get(t, ts, "/?view=all"))
	if !strings.Contains(page, "aria-current=\"page\">All") {
		t.Fatal("All tab should be active")
	}
	for _, want := range []string{"Blocked by open", "Blocked by closed", "Closed blocker"} {
		if !strings.Contains(page, want) {
			t.Errorf("expected %q in All view", want)
		}
	}
}

func TestList_BlockedView(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	page := body(t, get(t, ts, "/?view=blocked"))
	if !strings.Contains(page, "aria-current=\"page\">Blocked") {
		t.Fatal("Blocked tab should be active")
	}
	if !strings.Contains(page, "Blocked by open") {
		t.Fatal("Blocked view should include the issue blocked by an open issue")
	}
	if strings.Contains(page, "Open unblocked") {
		t.Fatal("Blocked view should exclude unblocked issues")
	}
}

func TestList_ReadyView(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)
	if err := s.UpdateIssue(5, store.UpdateIssueOptions{AddLabels: []string{"ready-for-agent"}}); err != nil {
		t.Fatal(err)
	}

	page := body(t, get(t, ts, "/?view=ready"))
	if !strings.Contains(page, "aria-current=\"page\">Ready") {
		t.Fatal("Ready tab should be active")
	}
	if !strings.Contains(page, "Open unblocked") {
		t.Fatal("Ready view should include the ready, unblocked issue")
	}
	if strings.Contains(page, "Blocked by open") {
		t.Fatal("blocked issue must not appear in Ready view")
	}
}

func TestList_KindFilter(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	page := body(t, get(t, ts, "/?kind=spec"))
	if !strings.Contains(page, "Open blocker") {
		t.Fatal("kind=spec should include the spec issue")
	}
	if strings.Contains(page, "Blocked by open") || strings.Contains(page, "Open unblocked") {
		t.Fatal("kind=spec should exclude task issues")
	}
}

func TestList_LabelFilter(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	page := body(t, get(t, ts, "/?label=enhancement"))
	if !strings.Contains(page, "Open unblocked") {
		t.Fatal("label=enhancement should include issues with that label")
	}
	if strings.Contains(page, "Blocked by open") {
		t.Fatal("label=enhancement should exclude issues without that label")
	}
}

func TestList_FiltersPreservedAcrossTabs(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	page := body(t, get(t, ts, "/?view=all&kind=task&label=enhancement"))
	for _, want := range []string{
		"href=\"/?view=open&amp;kind=task&amp;label=enhancement\"",
		"href=\"/?view=all&amp;kind=task&amp;label=enhancement\"",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("tab link should preserve filters, missing %s", want)
		}
	}
}

func TestList_BlockedBadgeOpenBlocker(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	page := body(t, get(t, ts, "/"))
	if !rowBadged(page, "Blocked by open") {
		t.Fatal("expected a blocked badge for the issue blocked by an open issue")
	}
}

func TestList_NoBadgeWhenBlockerClosed(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	// Issue 2 is blocked only by closed issue 4: it must NOT show a badge,
	// even though the All view also shows the genuinely blocked issue 1.
	page := body(t, get(t, ts, "/?view=all"))
	if !rowContains(page, "Blocked by closed") {
		t.Fatal("fixture issue missing from list")
	}
	if rowBadged(page, "Blocked by closed") {
		t.Fatal("issue blocked only by a closed issue must not show a blocked badge")
	}
	if !rowBadged(page, "Blocked by open") {
		t.Fatal("issue blocked by an open issue should still show the badge in All view")
	}
}

func TestList_Empty(t *testing.T) {
	ts, _ := newTestServer(t)

	page := body(t, get(t, ts, "/"))
	if !strings.Contains(page, "No issues found") {
		t.Fatal("expected empty state message")
	}
}

func TestHostGuard_LoopbackAccepted(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	for _, host := range []string{"127.0.0.1", "localhost", "::1", "127.0.0.2"} {
		req, err := http.NewRequest("GET", ts.URL+"/", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("loopback host %q should be accepted, got %d", host, resp.StatusCode)
		}
	}
}

func TestHostGuard_AttackerHostRejected(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "attacker.example.com"
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("attacker host should be rejected with 403, got %d", resp.StatusCode)
	}
	assertSecurityHeaders(t, resp)
}

func TestHostGuard_ExplicitBindHostAccepted(t *testing.T) {
	s, err := store.Ensure(filepath.Join(t.TempDir(), "litt.db"))
	if err != nil {
		t.Fatal(err)
	}
	// A concrete non-wildcard bind host is auto-allowed by the host guard.
	ts := httptest.NewServer(NewHandler(s, Options{BindHost: "192.168.1.5"}))
	defer ts.Close()
	defer s.Close()

	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "192.168.1.5"
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("explicit bind host should be accepted, got %d", resp.StatusCode)
	}
}

func TestHostGuard_AllowExternalDisablesGuard(t *testing.T) {
	s, err := store.Ensure(filepath.Join(t.TempDir(), "litt.db"))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewHandler(s, Options{BindHost: "0.0.0.0", AllowExternal: true}))
	defer ts.Close()
	defer s.Close()

	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "attacker.example.com"
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("AllowExternal should accept any host, got %d", resp.StatusCode)
	}
}

func TestSecurityHeaders(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	resp := get(t, ts, "/")
	defer resp.Body.Close()
	assertSecurityHeaders(t, resp)
}

func assertSecurityHeaders(t *testing.T, resp *http.Response) {
	t.Helper()
	if csp := resp.Header.Get("Content-Security-Policy"); csp == "" {
		t.Error("missing Content-Security-Policy header")
	} else if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP missing frame-ancestors: %s", csp)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff header")
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("missing frame-denial header")
	}
}

func TestSecurityHeaders_OnRejectedHost(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "attacker.example.com"
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	assertSecurityHeaders(t, resp)
}

func TestStatic_ServesCSS(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := get(t, ts, "/static/pico.css")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for pico.css, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("expected text/css, got %s", ct)
	}
}

func TestStatic_NotFound(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := get(t, ts, "/static/missing.css")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing asset, got %d", resp.StatusCode)
	}
}

func TestDisplayAddr(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"127.0.0.1:57664", "127.0.0.1:57664"},
		{"0.0.0.0:57664", "localhost:57664"},
		{"[::]:57664", "localhost:57664"},
		{":57664", "localhost:57664"},
		{"192.168.1.5:8080", "192.168.1.5:8080"},
	} {
		if got := DisplayAddr(tc.in); got != tc.want {
			t.Errorf("DisplayAddr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHostGuard_InvalidHostWithoutPort(t *testing.T) {
	ts, s := newTestServer(t)
	seedFixtures(t, s)

	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "attacker.example.com:1234"
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("attacker host with port should be rejected, got %d", resp.StatusCode)
	}
}
