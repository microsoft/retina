// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package server

import (
	"context"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// pprofPathPrefix is the URL prefix used by net/http/pprof handlers.
const pprofPathPrefix = "/debug/pprof/"

type Server struct {
	l   *log.ZapLogger
	mux *chi.Mux
}

func New(logger *log.ZapLogger) *Server {
	r := chi.NewRouter()
	r.Use(
		// pprofLoopbackOnly MUST run before middleware.RealIP so it inspects
		// the raw TCP peer address rather than a value derived from
		// X-Forwarded-For / X-Real-IP headers (which are attacker-controlled).
		pprofLoopbackOnly,
		middleware.RequestID,
		middleware.RealIP,
		middleware.Recoverer,
		middleware.Timeout(60*time.Second),
	)

	return &Server{
		l:   logger,
		mux: r,
	}
}

// pprofLoopbackOnly restricts /debug/pprof/* endpoints so they only respond
// to requests arriving on the loopback interface. Any request whose raw TCP
// remote address is not loopback (127.0.0.0/8 or ::1) receives a 404 — the
// same status an unregistered path would return, so external scanners cannot
// fingerprint the presence of pprof.
//
// This preserves the existing profiling workflow (`kubectl exec <pod> --
// curl localhost:10093/debug/pprof/...` and `kubectl port-forward`), both of
// which terminate on the pod's loopback interface, while blocking access
// from other pods, other nodes, and off-cluster peers.
//
// IMPORTANT: this middleware must be registered before middleware.RealIP;
// otherwise r.RemoteAddr would be rewritten from X-Forwarded-For and the
// loopback check would be trivially bypassable via a spoofed header.
func pprofLoopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, pprofPathPrefix) && !isLoopbackRemoteAddr(r.RemoteAddr) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLoopbackRemoteAddr returns true iff addr (in host:port form as provided
// by http.Request.RemoteAddr) is on the loopback interface.
func isLoopbackRemoteAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Fall back to treating the whole string as the host — some transports
		// (e.g. unix sockets) leave RemoteAddr without a port.
		host = addr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func (rt *Server) SetupHandlers() {
	rt.l.Info("Setting up handlers")
	rt.servePrometheusMetrics()
	exporter.RegisterMetricsServeCallback(func() {
		rt.servePrometheusMetrics()
	})
	rt.mux.HandleFunc("/debug/pprof/", pprof.Index)
	rt.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	rt.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	rt.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	rt.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	rt.mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	rt.mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	rt.mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	rt.mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	rt.mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	rt.l.Info("Completed handler setup")
}

func (rt *Server) servePrometheusMetrics() {
	rt.mux.Get("/metrics", promhttp.HandlerFor(exporter.CombinedGatherer, promhttp.HandlerOpts{}).ServeHTTP)
}

func (rt *Server) Start(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: rt.mux}
	g, gctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		rt.l.Info("starting HTTP server... on ", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil {
			rt.l.Sugar().Infof("HTTP server stopped with err: %v", err)
			return err
		}
		return nil
	})

	select {
	case <-ctx.Done():
		rt.l.Info("gracefully shutting down HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to gracefully shutdown HTTP server")
		}

		// wait for listenAndServe to return
		<-gctx.Done()

	case <-gctx.Done():
		return errors.Wrapf(gctx.Err(), "failed to start HTTP server")
	}

	return nil
}
