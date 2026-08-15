package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestDetail_AddBlockingRedirectAndEffect(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("Dependency", "spec", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	assertRedirect(t, post(t, ts, "/issues/1/blocking", url.Values{"csrf": {csrf}, "target": {"2"}}), "/issues/1")
	blocking, err := s.ListBlocking(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocking) != 1 || blocking[0].ID != 2 {
		t.Fatalf("expected issue 1 to block issue 2, got %+v", blocking)
	}
}

func TestDetail_AddBlockedByRedirectAndEffect(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("Blocker", "spec", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	assertRedirect(t, post(t, ts, "/issues/1/blocked-by", url.Values{"csrf": {csrf}, "target": {"2"}}), "/issues/1")
	blockedBy, err := s.ListBlockedBy(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockedBy) != 1 || blockedBy[0].ID != 2 {
		t.Fatalf("expected issue 1 to be blocked by issue 2, got %+v", blockedBy)
	}
}

func TestDetail_RemoveBlockingRedirectAndEffect(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("Dependency", "spec", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	assertRedirect(t, post(t, ts, "/issues/1/blocking/remove", url.Values{"csrf": {csrf}, "target": {"2"}}), "/issues/1")
	blocking, err := s.ListBlocking(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocking) != 0 {
		t.Fatalf("expected block edge removed, got %+v", blocking)
	}
}

func TestDetail_RemoveBlockedByRedirectAndEffect(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("Blocker", "spec", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(2, 1); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	assertRedirect(t, post(t, ts, "/issues/1/blocked-by/remove", url.Values{"csrf": {csrf}, "target": {"2"}}), "/issues/1")
	blockedBy, err := s.ListBlockedBy(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockedBy) != 0 {
		t.Fatalf("expected blocked-by edge removed, got %+v", blockedBy)
	}
}

func TestDetail_SelfBlockRejected(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Lonely", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	for _, path := range []string{"/issues/1/blocking", "/issues/1/blocked-by"} {
		resp := post(t, ts, path, url.Values{"csrf": {csrf}, "target": {"1"}})
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("POST %s self-block: expected 422, got %d", path, resp.StatusCode)
		}
		if page := body(t, resp); !strings.Contains(page, "cannot block or be blocked by itself") {
			t.Fatalf("POST %s self-block should surface the rejection, got page without message", path)
		}
	}

	blocking, err := s.ListBlocking(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocking) != 0 {
		t.Fatalf("self-block must not create an edge, got %+v", blocking)
	}
}

func TestDetail_DuplicateBlockRejected(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("Dependency", "spec", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	resp := post(t, ts, "/issues/1/blocking", url.Values{"csrf": {csrf}, "target": {"2"}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for duplicate edge, got %d", resp.StatusCode)
	}
	if page := body(t, resp); !strings.Contains(page, "already blocks #2") {
		t.Fatal("duplicate edge should surface a rejection message")
	}
	blocking, err := s.ListBlocking(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocking) != 1 {
		t.Fatalf("duplicate edge must not create a second edge, got %+v", blocking)
	}
}

func TestDetail_RemoveMissingEdgeRejected(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("Dependency", "spec", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	resp := post(t, ts, "/issues/1/blocking/remove", url.Values{"csrf": {csrf}, "target": {"2"}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing edge removal, got %d", resp.StatusCode)
	}
	if page := body(t, resp); !strings.Contains(page, "Block edge not found") {
		t.Fatal("missing edge removal should surface a rejection message")
	}
}

func TestDetail_EdgeWriteUnknownTargetRejected(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 1)

	resp := post(t, ts, "/issues/1/blocking", url.Values{"csrf": {csrf}, "target": {"999"}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for unknown target, got %d", resp.StatusCode)
	}
	if page := body(t, resp); !strings.Contains(page, "Issue #999 not found") {
		t.Fatal("unknown target should surface a rejection message")
	}
	if blocking, err := s.ListBlocking(1); err != nil {
		t.Fatal(err)
	} else if len(blocking) != 0 {
		t.Fatalf("unknown target must not create an edge, got %+v", blocking)
	}
}

func TestDetail_EdgeWriteRejectedWithoutCSRF(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("Dependency", "spec", "", nil); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		form url.Values
	}{
		{"/issues/1/blocking", url.Values{"target": {"2"}}},
		{"/issues/1/blocked-by", url.Values{"target": {"2"}}},
		{"/issues/1/blocking/remove", url.Values{"target": {"2"}}},
		{"/issues/1/blocked-by/remove", url.Values{"target": {"2"}}},
	} {
		resp := post(t, ts, tc.path, tc.form)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("POST %s without csrf: expected 403, got %d", tc.path, resp.StatusCode)
		}
	}

	if blocking, err := s.ListBlocking(1); err != nil {
		t.Fatal(err)
	} else if len(blocking) != 0 {
		t.Fatalf("rejected writes must not create edges, got %+v", blocking)
	}
	if blockedBy, err := s.ListBlockedBy(1); err != nil {
		t.Fatal(err)
	} else if len(blockedBy) != 0 {
		t.Fatalf("rejected writes must not create edges, got %+v", blockedBy)
	}
}

func TestDetail_CycleEdgeRejected(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("A", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIssue("B", "task", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}
	csrf := csrfToken(t, ts, 2)

	resp := post(t, ts, "/issues/2/blocking", url.Values{"csrf": {csrf}, "target": {"1"}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for cycle-forming edge, got %d", resp.StatusCode)
	}
	if page := body(t, resp); !strings.Contains(page, "create a cycle") {
		t.Fatal("cycle-forming edge should surface a rejection message")
	}
	blocking, err := s.ListBlocking(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocking) != 0 {
		t.Fatalf("cycle-forming edge must not be created, got %+v", blocking)
	}
}

func TestDetail_EdgeAddFormRendered(t *testing.T) {
	ts, s := newTestServer(t)
	if _, err := s.CreateIssue("Main", "task", "", nil); err != nil {
		t.Fatal(err)
	}

	page := body(t, get(t, ts, "/issues/1"))
	for _, want := range []string{`action="/issues/1/blocking"`, `action="/issues/1/blocked-by"`, `name="target"`} {
		if !strings.Contains(page, want) {
			t.Errorf("detail page missing %q", want)
		}
	}
}
