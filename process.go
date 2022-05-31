package svcrunner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/go-logr/logr"
	"go.seankhliao.com/svcrunner/envflag"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

type Process struct {
	Name     string
	Register RegFunc
	Init     RunFunc
	Start    RunFunc
	Stop     RunFunc
}

type (
	RunFunc func(context.Context, Tools) error
	RegFunc func(*envflag.Config)
)

func NewHTTP(s *http.Server, reg RegFunc, init RunFunc) Process {
	// configure for h2c
	s.Handler = h2c.NewHandler(s.Handler, &http2.Server{})

	var host, port string
	return Process{
		Register: func(c *envflag.Config) {
			c.StringVar(&host, "host", "", "host to bind bind to")
			c.StringVar(&port, "port", "8080", "port to listen on")
			if reg != nil {
				reg(c)
			}
		},
		Init: init,
		Start: func(ctx context.Context, t Tools) error {
			if s.ErrorLog == nil {
				s.ErrorLog = log.New(&logWriter{t.Log.WithName("http")}, "", 0)
			}
			s.Addr = net.JoinHostPort(host, port)
			t.Log.Info("starting http server", "addr", s.Addr)
			err := s.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("svcrunner http serve: %w", err)
			}
			return nil
		},
		Stop: func(ctx context.Context, t Tools) error {
			err := s.Shutdown(ctx)
			if err != nil {
				return fmt.Errorf("svcrunner http shutdown: %w", err)
			}
			return nil
		},
	}
}

func NewGRPC(s *grpc.Server, reg RegFunc, init RunFunc) Process {
	var host, port string
	return Process{
		Register: func(c *envflag.Config) {
			c.StringVar(&host, "host", "", "host to bind bind to")
			c.StringVar(&port, "port", "8080", "port to listen on")
			if reg != nil {
				reg(c)
			}
		},
		Init: init,
		Start: func(ctx context.Context, t Tools) error {
			addr := net.JoinHostPort(host, port)
			t.Log.Info("starting http server", "addr", addr)
			lis, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("svcrunner grpc listen %v: %w", addr, err)
			}
			err = s.Serve(lis)
			if err != nil {
				return fmt.Errorf("svcrunner grpc serve: %w", err)
			}
			return nil
		},
		Stop: func(ctx context.Context, t Tools) error {
			go func() {
				<-ctx.Done()
				s.Stop()
			}()
			s.GracefulStop()
			return nil
		},
	}
}

type logWriter struct {
	log logr.Logger
}

func (l *logWriter) Write(b []byte) (int, error) {
	l.log.Info(string(b))
	return len(b), nil
}
