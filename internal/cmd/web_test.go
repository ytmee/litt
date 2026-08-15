package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
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
		done <- runWeb(ctx, s, "127.0.0.1:0", true, &out)
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
	go func() { done <- runWeb(ctx, s, defaultWebAddr, false, &out) }()

	// Use the real default port; a busy 57664 bumps to 57665..57673, so the
	// assertion accepts the whole default bump range.
	url := waitForURL(t, &out, 5*time.Second)
	_, portStr, err := net.SplitHostPort(strings.TrimPrefix(url, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	if port < 57664 || port > 57673 {
		t.Fatalf("expected port in 57664..57673, got %d", port)
	}

	resp, err := http.Get(url)
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

	err = runWeb(context.Background(), s, ln.Addr().String(), true, io.Discard)
	if err == nil {
		t.Fatal("expected error for taken port")
	}
	if !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected the underlying listen error, got: %v", err)
	}
}

func TestRunWeb_DefaultBumpsWhenBusy(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	preferred := busy.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- runWeb(ctx, s, preferred, false, &out) }()

	url := waitForURL(t, &out, 5*time.Second)
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("expected localhost URL, got %q", url)
	}
	if strings.HasSuffix(url, portOf(preferred)) {
		t.Fatalf("expected bumped port, got busy port %q", url)
	}
	if !strings.Contains(out.String(), "busy") {
		t.Fatalf("expected busy notice in output: %q", out.String())
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on bumped port, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestRunWeb_AllCandidatesBusy(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	probe.Close()
	preferred := probe.Addr().String()
	_, portStr, err := net.SplitHostPort(preferred)
	if err != nil {
		t.Fatal(err)
	}
	base, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	busy := make([]net.Listener, 0, maxWebPortAttempts)
	defer func() {
		for _, ln := range busy {
			ln.Close()
		}
	}()
	for i := 0; i < maxWebPortAttempts; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", base+i))
		if err != nil {
			t.Fatalf("occupy port %d: %v", base+i, err)
		}
		busy = append(busy, ln)
	}

	err = runWeb(context.Background(), s, preferred, false, io.Discard)
	if err == nil {
		t.Fatal("expected error when all candidate ports are busy")
	}
	if !strings.Contains(err.Error(), "candidate ports busy") {
		t.Fatalf("expected exhaustion error, got: %v", err)
	}
}

func TestRunWeb_FailFastNonAddrInUse(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()

	// An unresolvable host is not EADDRINUSE, so a non-strict bind must fail
	// immediately rather than bumping through a bad candidate list.
	err := runWeb(context.Background(), s, "no-such-host.example:57664", false, io.Discard)
	if err == nil {
		t.Fatal("expected error for unresolvable host")
	}
	if strings.Contains(err.Error(), "candidate ports busy") {
		t.Fatalf("expected immediate failure, got exhaustion error: %v", err)
	}
}

func TestRunWeb_HostGuardActiveByDefault(t *testing.T) {
	s := webTestStore(t)
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- runWeb(ctx, s, "127.0.0.1:0", true, &out) }()

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
	go func() { done <- runWeb(ctx, s, "127.0.0.1:0", true, &out) }()

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

func TestWebListenAddrs(t *testing.T) {
	t.Run("strict returns only the preferred address", func(t *testing.T) {
		addrs, err := webListenAddrs("127.0.0.1:57664", true)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"127.0.0.1:57664"}; !reflect.DeepEqual(addrs, want) {
			t.Fatalf("got %v, want %v", addrs, want)
		}
	})

	t.Run("non-strict bumps up to 10 ports preserving the host", func(t *testing.T) {
		addrs, err := webListenAddrs("127.0.0.1:57664", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(addrs) != maxWebPortAttempts {
			t.Fatalf("got %d candidates, want %d", len(addrs), maxWebPortAttempts)
		}
		for i, a := range addrs {
			if want := fmt.Sprintf("127.0.0.1:%d", 57664+i); a != want {
				t.Fatalf("candidate %d = %q, want %q", i, a, want)
			}
		}
	})

	t.Run("non-strict errors on a malformed address", func(t *testing.T) {
		if _, err := webListenAddrs("not-an-address", false); err == nil {
			t.Fatal("expected error for malformed address")
		}
	})

	t.Run("non-strict preserves a non-loopback host", func(t *testing.T) {
		addrs, err := webListenAddrs("0.0.0.0:57664", false)
		if err != nil {
			t.Fatal(err)
		}
		if want := "0.0.0.0:57664"; addrs[0] != want {
			t.Fatalf("first candidate = %q, want %q", addrs[0], want)
		}
		for _, a := range addrs {
			if !strings.HasPrefix(a, "0.0.0.0:") {
				t.Fatalf("candidate %q changed host", a)
			}
		}
	})

	t.Run("non-strict clamps to the valid port range", func(t *testing.T) {
		addrs, err := webListenAddrs("127.0.0.1:65533", false)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"127.0.0.1:65533", "127.0.0.1:65534", "127.0.0.1:65535"}; !reflect.DeepEqual(addrs, want) {
			t.Fatalf("got %v, want %v", addrs, want)
		}
	})
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
