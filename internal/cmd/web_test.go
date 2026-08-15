package cmd

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ytmee/litt/internal/store"
)

func webTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Ensure(filepath.Join(t.TempDir(), "litt.db"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// waitForURL polls out for a printed http:// URL until timeout.
func waitForURL(t *testing.T, out *bytes.Buffer, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content := out.String()
		if idx := strings.Index(content, "http://"); idx != -1 {
			rest := content[idx:]
			if end := strings.IndexAny(rest, " \n\t\r"); end != -1 {
				rest = rest[:end]
			}
			return rest
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no URL printed within %s; output so far: %q", timeout, out.String())
	return ""
}

func TestWeb_NoRepo(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	_, err := runCmd(t, "web")
	if err == nil {
		t.Fatal("expected error for uninitialized repo")
	}
	if !strings.Contains(err.Error(), "not a litt repository") {
		t.Fatalf("expected clear error, got: %v", err)
	}
}

func TestWeb_ExplicitDBFlag(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	customDB := filepath.Join(dir, "custom", "data.db")
	if _, err := runCmd(t, "init", "--db", customDB); err != nil {
		t.Fatal(err)
	}

	// `litt web --db` resolves the DB but must still fail on port conflict,
	// proving the flag is wired through openStore.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	_, err = runCmd(t, "web", "--db", customDB, "--addr", ln.Addr().String())
	if err == nil {
		t.Fatal("expected error for taken port")
	}
	if !strings.Contains(err.Error(), "listen") {
		t.Fatalf("expected listen error, got: %v", err)
	}
}

func TestRunWeb_ServesListPageAndShutsDown(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()
	if _, err := s.CreateIssue("Hello web", "task", "", nil); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runWeb(ctx, s, "127.0.0.1:0", &out)
	}()

	url := waitForURL(t, &out, 5*time.Second)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(data), "Hello web") {
		t.Fatalf("list page missing fixture issue: %s", data)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graceful shutdown returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runWeb did not return after context cancel")
	}
}

func TestRunWeb_PrintsURLForDefaultAddr(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- runWeb(ctx, s, defaultWebAddr, &out) }()

	// Use the real default port; if something else grabbed it, the error path
	// still surfaces in done and the test fails on the port check below.
	url := waitForURL(t, &out, 5*time.Second)
	if !strings.HasPrefix(url, "http://127.0.0.1:57664") {
		t.Fatalf("expected default addr URL, got %q", url)
	}

	resp, err := http.Get("http://127.0.0.1:57664/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on default port, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestRunWeb_PortTaken(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	err = runWeb(context.Background(), s, ln.Addr().String(), io.Discard)
	if err == nil {
		t.Fatal("expected error for taken port")
	}
	if !strings.Contains(err.Error(), "listen") {
		t.Fatalf("expected listen error, got: %v", err)
	}
}

func TestRunWeb_HostGuardActiveByDefault(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- runWeb(ctx, s, "127.0.0.1:0", &out) }()

	url := waitForURL(t, &out, 5*time.Second)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "attacker.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for attacker host, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestRunWeb_ExposeEnvDisablesGuard(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()

	t.Setenv(webExposeEnv, "1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- runWeb(ctx, s, "127.0.0.1:0", &out) }()

	url := waitForURL(t, &out, 5*time.Second)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "attacker.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with expose env set, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestWeb_HelpMentionsAddrDefault(t *testing.T) {
	root := NewRootCmd("test")
	root.SetArgs([]string{"web", "--help"})
	var out bytes.Buffer
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), defaultWebAddr) {
		t.Fatalf("help should mention default addr %s, got: %s", defaultWebAddr, out.String())
	}
	if !strings.Contains(out.String(), webExposeEnv) {
		t.Fatalf("help should mention %s, got: %s", webExposeEnv, out.String())
	}
}
