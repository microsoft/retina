// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package servermanager

import (
	"context"
	"fmt"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/server"
	"go.uber.org/zap"
)

type HTTPServer struct {
	l      *log.ZapLogger
	host   string
	port   int
	router *server.Server
}

func NewHTTPServer(
	host string, port int,
) *HTTPServer {
	logger := log.Logger().Named("http-server")
	return &HTTPServer{
		l:    logger,
		host: host,
		port: port,
	}
}

func (s *HTTPServer) Init() error {
	s.l.Info("Initializing HTTP server ...")
	rt := server.New(s.l)
	rt.SetupHandlers()
	s.router = rt
	s.l.Info("HTTP server initialized...")
	return nil
}

func (s *HTTPServer) Start(ctx context.Context) error {
	s.l.Info("Starting HTTP server ...", zap.String("host", s.host), zap.Int("port", s.port))
	return s.router.Start(ctx, fmt.Sprintf("%s:%d", s.host, s.port))
}
