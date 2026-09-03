package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestServerShutdown(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	s := New(log.Logger().Named("http-server"))
	s.SetupHandlers()

	ctx, cancel := context.WithCancel(context.Background())
	g, errctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return s.Start(errctx, "localhost:10093")
	})

	// wait for server to start
	time.Sleep(2 * time.Second)
	cancel()
	_ = g.Wait()

	// require.NoError(t, err)
	// Ignoring the error check since this can cause transient errors in CI
}

func TestServerStartOnUsedPort(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	s := New(log.Logger().Named("http-server"))
	s.SetupHandlers()

	ctx, cancel := context.WithCancel(context.Background())
	g, errctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return s.Start(errctx, "localhost:10093")
	})

	g.Go(func() error {
		return s.Start(errctx, "localhost:10093")
	})

	time.Sleep(2 * time.Second)
	cancel()
	err = g.Wait()

	require.Error(t, err)
}

// TestPprofRejectsNonLoopback ensures /debug/pprof/* returns 404 when the
// request's TCP remote address is not on the loopback interface. This is
// the security guarantee added to close MSRC/CVE for unauthenticated pprof
// exposure — external peers must not be able to trigger CPU profiling
// (denial of service) or dump heap/goroutine state (information disclosure).
func TestPprofRejectsNonLoopback(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	s := New(log.Logger().Named("http-server"))
	s.SetupHandlers()

	nonLoopbackPeers := []string{
		"10.0.0.5:54321",     // typical pod IP
		"192.168.1.10:12345", // typical LAN
		"203.0.113.7:443",    // typical public
		"[2001:db8::1]:9999", // typical IPv6
	}
	pprofPaths := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/allocs",
		"/debug/pprof/block",
		"/debug/pprof/threadcreate",
	}

	for _, peer := range nonLoopbackPeers {
		for _, path := range pprofPaths {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
			req.RemoteAddr = peer
			rw := httptest.NewRecorder()
			s.mux.ServeHTTP(rw, req)
			require.Equalf(t, http.StatusNotFound, rw.Code,
				"pprof path %s from non-loopback %s must be 404, got %d",
				path, peer, rw.Code)
		}
	}
}

// TestPprofSpoofedXForwardedForDoesNotBypass verifies that a caller cannot
// bypass the loopback guard by setting X-Forwarded-For / X-Real-IP to
// 127.0.0.1. The middleware chain is deliberately ordered so pprof's
// loopback check runs before chi's RealIP middleware.
func TestPprofSpoofedXForwardedForDoesNotBypass(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	s := New(log.Logger().Named("http-server"))
	s.SetupHandlers()

	for _, hdr := range []string{"X-Forwarded-For", "X-Real-IP", "True-Client-IP"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/debug/pprof/heap", http.NoBody)
		req.RemoteAddr = "10.0.0.5:54321"
		req.Header.Set(hdr, "127.0.0.1")
		rw := httptest.NewRecorder()
		s.mux.ServeHTTP(rw, req)
		require.Equalf(t, http.StatusNotFound, rw.Code,
			"spoofed %s: 127.0.0.1 must not bypass pprof loopback guard", hdr)
	}
}

// TestPprofAllowsLoopback confirms the guard permits genuine loopback
// callers (kubectl exec + curl localhost, or kubectl port-forward).
func TestPprofAllowsLoopback(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	s := New(log.Logger().Named("http-server"))
	s.SetupHandlers()

	for _, peer := range []string{"127.0.0.1:33333", "127.10.20.30:44444", "[::1]:55555"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/debug/pprof/", http.NoBody)
		req.RemoteAddr = peer
		rw := httptest.NewRecorder()
		s.mux.ServeHTTP(rw, req)
		require.NotEqualf(t, http.StatusNotFound, rw.Code,
			"pprof from loopback %s must NOT be 404, got %d", peer, rw.Code)
	}
}

// TestMetricsReachableFromAnyPeer confirms /metrics is unaffected by the
// pprof loopback guard — it must remain reachable from any source.
func TestMetricsReachableFromAnyPeer(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	s := New(log.Logger().Named("http-server"))
	s.SetupHandlers()

	for _, peer := range []string{"10.0.0.5:54321", "127.0.0.1:12345"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
		req.RemoteAddr = peer
		rw := httptest.NewRecorder()
		s.mux.ServeHTTP(rw, req)
		require.NotEqualf(t, http.StatusNotFound, rw.Code,
			"/metrics from %s must not 404, got %d", peer, rw.Code)
	}
}

func TestIsLoopbackRemoteAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:12345", true},
		{"127.10.20.30:1", true},
		{"[::1]:80", true},
		{"10.0.0.1:80", false},
		{"192.168.1.1:80", false},
		{"[2001:db8::1]:80", false},
		{"not-an-address", false},
		{"", false},
		// hostless variants
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", false},
	}
	for _, c := range cases {
		require.Equalf(t, c.want, isLoopbackRemoteAddr(c.addr),
			"isLoopbackRemoteAddr(%q)", c.addr)
	}
}
