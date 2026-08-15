package web

import (
	"net"
	"net/http"
	"strings"
)

// DisplayAddr rewrites wildcard bind addresses to localhost so the printed URL
// is always openable in a browser.
func DisplayAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if isWildcardHost(host) {
		host = "localhost"
	}
	return net.JoinHostPort(host, port)
}

// BindHost returns the host portion of a bind address, or the address itself
// when it has no port.
func BindHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func isWildcardHost(host string) bool {
	switch host {
	case "", "0.0.0.0", "::":
		return true
	}
	return false
}

func bindHostAllowlist(bindHost string) []string {
	if isWildcardHost(bindHost) {
		return nil
	}
	return []string{bindHost}
}

func hostname(hostport string) string {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	return hostport
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(host)
	switch host {
	case "localhost", "localhost.localdomain", "ip6-localhost", "ip6-loopback":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func hostGuard(extra []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(extra))
	for _, host := range extra {
		allowed[strings.ToLower(host)] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := hostname(r.Host)
			if isLoopbackHost(host) || allowed[strings.ToLower(host)] {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "forbidden: unexpected Host header", http.StatusForbidden)
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
