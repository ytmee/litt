package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/store"
	"github.com/ytmee/litt/internal/web"
)

const defaultWebAddr = "127.0.0.1:57664"

// maxWebPortAttempts bounds how many consecutive ports a non-strict web bind
// tries before giving up. Enough headroom for several litt projects serving
// concurrently on one machine.
const maxWebPortAttempts = 10

// webExposeEnv acknowledges deliberate exposure of the WebUI beyond loopback.
const webExposeEnv = "LITT_WEB_EXPOSE"

func newWebCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the litt web UI",
		Long: `Start a single-serving HTTP server for the litt WebUI.

The UI listens on 127.0.0.1:57664 by default (override with --addr) and prints
an openable localhost URL. If the default port is busy — e.g. another litt
project is already serving — it bumps to the next free port and reports the
actual address; an explicit --addr binds strictly and fails if taken. Requests
with an unexpected Host header are rejected to prevent DNS-rebinding attacks;
set ` + webExposeEnv + ` to deliberately expose the UI beyond loopback. Press
Ctrl+C to stop.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runWeb(ctx, s, addr, cmd.Flags().Changed("addr"), cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&addr, "addr", defaultWebAddr, "bind address for the web UI")
	return cmd
}

// webListenAddrs returns the ordered list of addresses to attempt when binding
// the web server. A strict bind (explicit --addr) tries only the preferred
// address; a non-strict bind (default) bumps the port up to
// maxWebPortAttempts times, preserving the host, so several litt projects can
// serve concurrently on one machine.
func webListenAddrs(preferred string, strict bool) ([]string, error) {
	if strict {
		return []string{preferred}, nil
	}
	host, portStr, err := net.SplitHostPort(preferred)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address %q: %w", preferred, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port in listen address %q: %w", preferred, err)
	}
	addrs := make([]string, 0, maxWebPortAttempts)
	for i := 0; i < maxWebPortAttempts && port+i <= 65535; i++ {
		addrs = append(addrs, net.JoinHostPort(host, strconv.Itoa(port+i)))
	}
	return addrs, nil
}

// isAddrInUse reports whether err is a "address already in use" bind failure.
func isAddrInUse(err error) bool {
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	return errors.Is(opErr.Err, syscall.EADDRINUSE)
}

// portOf returns the port portion of a host:port address, or "" on error.
func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}

// runWeb serves the WebUI until ctx is cancelled, then shuts down gracefully.
// A strict bind (explicit --addr) fails on a busy port; a non-strict bind
// bumps to the next free port and reports the actual address.
func runWeb(ctx context.Context, s *store.Store, addr string, strict bool, out io.Writer) error {
	bindHost := web.BindHost(addr)
	handler := web.NewHandler(s, web.Options{
		BindHost:      bindHost,
		AllowExternal: os.Getenv(webExposeEnv) != "",
	})

	addrs, err := webListenAddrs(addr, strict)
	if err != nil {
		return err
	}

	var ln net.Listener
	var lastErr error
	for _, a := range addrs {
		ln, err = net.Listen("tcp", a)
		if err == nil {
			break
		}
		if !isAddrInUse(err) {
			return fmt.Errorf("listen on %s: %w", a, err)
		}
		lastErr = err
	}
	if ln == nil {
		if strict {
			return fmt.Errorf("listen on %s: %w", addrs[0], lastErr)
		}
		return fmt.Errorf("listen on %s: all %d candidate ports busy", addr, len(addrs))
	}
	defer ln.Close()

	display := ln.Addr().String()
	if p := portOf(addr); p != "" && p != "0" && p != portOf(display) {
		fmt.Fprintf(out, "litt web: %s busy; using %s\n", addr, display)
	}
	fmt.Fprintf(out, "litt web serving at http://%s\n", web.DisplayAddr(display))

	srv := &http.Server{Handler: handler}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
