package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/store"
	"github.com/ytmee/litt/internal/web"
)

const defaultWebAddr = "127.0.0.1:57664"

// webExposeEnv acknowledges deliberate exposure of the WebUI beyond loopback.
const webExposeEnv = "LITT_WEB_EXPOSE"

func newWebCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the litt web UI",
		Long: `Start a single-serving HTTP server for the litt WebUI.

The UI listens on 127.0.0.1:57664 by default (override with --addr) and prints
an openable localhost URL. Requests with an unexpected Host header are rejected
to prevent DNS-rebinding attacks; set ` + webExposeEnv + ` to deliberately
expose the UI beyond loopback. Press Ctrl+C to stop.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runWeb(ctx, s, addr, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&addr, "addr", defaultWebAddr, "bind address for the web UI")
	return cmd
}

// runWeb serves the WebUI until ctx is cancelled, then shuts down gracefully.
func runWeb(ctx context.Context, s *store.Store, addr string, out io.Writer) error {
	bindHost := web.BindHost(addr)
	handler := web.NewHandler(s, web.Options{
		BindHost:      bindHost,
		AllowExternal: os.Getenv(webExposeEnv) != "",
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	defer ln.Close()

	display := ln.Addr().String()
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
