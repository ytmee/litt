package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newCSRF reads the synchronizer token from the create form so write tests can
// replay it in their POST bodies.
func newCSRF(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	return csrfFromPage(t, ts, "/issue/new")
}

func TestNewForm_RendersFields(t *testing.T) {
	ts, _ := newTestServer(t)

	page := body(t, get(t, ts, "/issue/new"))
	for _, want := range []string{`name="title"`, `name="kind"`, `name="body"`, `name="labels"`, `method="post" action="/issue/new"`} {
		if !strings.Contains(page, want) {
			t.Errorf("create form missing %q", want)
		}
	}
	if !strings.Contains(page, `value="enhancement"`) {
		t.Error("create form should list seeded labels")
	}
}

func TestList_HasNewIssueEntry(t *testing.T) {
	ts, _ := newTestServer(t)

	page := body(t, get(t, ts, "/"))
	if !strings.Contains(page, `href="/issue/new"`) {
		t.Fatal("list page should link to the new-issue form")
	}
}

func TestCreateIssue_RedirectsToDetail(t *testing.T) {
	ts, s := newTestServer(t)
	csrf := newCSRF(t, ts)

	assertRedirect(t, post(t, ts, "/issue/new", url.Values{
		"csrf":   {csrf},
		"title":  {"A new task"},
		"kind":   {"task"},
		"body":   {"body text"},
		"labels": {"enhancement"},
	}), "/issues/1")

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Title != "A new task" || issue.Kind != "task" || issue.Body != "body text" {
		t.Fatalf("created issue mismatch: %+v", issue)
	}
	if len(issue.Labels) != 1 || issue.Labels[0].Name != "enhancement" {
		t.Fatalf("labels not attached on create: %+v", issue.Labels)
	}
}

func TestCreateIssue_BlankTitleSurfacesValidationError(t *testing.T) {
	ts, s := newTestServer(t)
	csrf := newCSRF(t, ts)

	resp := post(t, ts, "/issue/new", url.Values{
		"csrf":  {csrf},
		"title": {"   "},
		"kind":  {"task"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for blank title, got %d", resp.StatusCode)
	}
	page := body(t, resp)
	if !strings.Contains(page, "Title is required.") {
		t.Fatal("blank title should surface a validation message, not a server error")
	}
	if !strings.Contains(page, `name="title"`) {
		t.Fatal("create form should be re-rendered so the user can correct it")
	}
	if issues, err := s.ListIssues("", "", "", nil, nil); err != nil {
		t.Fatal(err)
	} else if len(issues) != 0 {
		t.Fatalf("blank title must not create an issue, found %d", len(issues))
	}
}

func TestCreateIssue_InvalidKindSurfacesValidationError(t *testing.T) {
	ts, s := newTestServer(t)
	csrf := newCSRF(t, ts)

	resp := post(t, ts, "/issue/new", url.Values{
		"csrf":  {csrf},
		"title": {"Bogus kind"},
		"kind":  {"bogus"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid kind, got %d", resp.StatusCode)
	}
	if page := body(t, resp); !strings.Contains(page, "must be one of spec, task, or bug") {
		t.Fatal("invalid kind should surface a validation message, not a server error")
	}
	if issues, err := s.ListIssues("", "", "", nil, nil); err != nil {
		t.Fatal(err)
	} else if len(issues) != 0 {
		t.Fatalf("invalid kind must not create an issue, found %d", len(issues))
	}
}

func TestCreateIssue_UnknownLabelSurfacesValidationError(t *testing.T) {
	ts, s := newTestServer(t)
	csrf := newCSRF(t, ts)

	resp := post(t, ts, "/issue/new", url.Values{
		"csrf":   {csrf},
		"title":  {"Bad label"},
		"kind":   {"task"},
		"labels": {"no-such-label"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for unknown label, got %d", resp.StatusCode)
	}
	if page := body(t, resp); !strings.Contains(page, "no-such-label") {
		t.Fatal("unknown label should surface a validation message, not a server error")
	}
	if issues, err := s.ListIssues("", "", "", nil, nil); err != nil {
		t.Fatal(err)
	} else if len(issues) != 0 {
		t.Fatalf("unknown label must not create an issue, found %d", len(issues))
	}
}

func TestCreateIssue_RejectedWithoutCSRF(t *testing.T) {
	ts, s := newTestServer(t)

	resp := post(t, ts, "/issue/new", url.Values{"title": {"No csrf"}, "kind": {"task"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf, got %d", resp.StatusCode)
	}
	if issues, err := s.ListIssues("", "", "", nil, nil); err != nil {
		t.Fatal(err)
	} else if len(issues) != 0 {
		t.Fatalf("rejected create must not create an issue, found %d", len(issues))
	}
}
