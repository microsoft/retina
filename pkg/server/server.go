// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package server

import (
	"context"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

type Server struct {
	l   *log.ZapLogger
	mux *chi.Mux
}

func New(logger *log.ZapLogger) *Server {
	r := chi.NewRouter()
	r.Use(
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

func (rt *Server) SetupHandlers() {
	rt.l.Info("Setting up handlers")
	rt.servePrometheusMetrics()
	exporter.RegisterMetricsServeCallback(func() {
		rt.servePrometheusMetrics()
	})
	rt.serveHealth()
	rt.serveHealth2()
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

func (rt *Server) serveHealth() {
	rt.mux.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		rt.l.Error("serving health writing 20Ok")
	})
}

func (rt *Server) serveHealth2() {
	rt.mux.Get("/health2", healthz.CheckHandler{Checker: healthz.Ping}.ServeHTTP)
	rt.l.Error("serving health2 with Ping")
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
