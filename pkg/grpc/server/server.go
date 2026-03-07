package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	coreapp "github.com/nimburion/nimburion/pkg/core/app"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const defaultShutdownTimeout = 30 * time.Second

// Registration registers services on a gRPC server.
type Registration struct {
	Name string
	Fn   func(*grpc.Server) error
}

// LifecycleHook defines one named startup or shutdown action.
type LifecycleHook struct {
	Name string
	Fn   func(context.Context) error
}

// Config holds runtime gRPC server settings.
type Config struct {
	Address         string
	ShutdownTimeout time.Duration
	TLSConfig       *tls.Config
}

// Options configures gRPC build and runtime behavior.
type Options struct {
	Config Config

	Logger logger.Logger

	Listener net.Listener

	Registrations      []Registration
	UnaryInterceptors  []grpc.UnaryServerInterceptor
	StreamInterceptors []grpc.StreamServerInterceptor

	StartupHooks  []LifecycleHook
	ShutdownHooks []LifecycleHook

	AppName string
}

// Server owns a configured gRPC server and its listener.
type Server struct {
	config   Config
	logger   logger.Logger
	server   *grpc.Server
	listener net.Listener
}

// Build creates a gRPC server from options and service registrations.
func Build(opts Options) (*Server, error) {
	log := opts.Logger
	if log == nil {
		defaultLogger, err := logger.NewZapLogger(logger.DefaultConfig())
		if err != nil {
			return nil, fmt.Errorf("create default logger: %w", err)
		}
		log = defaultLogger
	}
	if opts.Config.Address == "" && opts.Listener == nil {
		return nil, errors.New("grpc address or listener is required")
	}

	serverOptions := make([]grpc.ServerOption, 0, 3)
	if len(opts.UnaryInterceptors) > 0 {
		serverOptions = append(serverOptions, grpc.ChainUnaryInterceptor(opts.UnaryInterceptors...))
	}
	if len(opts.StreamInterceptors) > 0 {
		serverOptions = append(serverOptions, grpc.ChainStreamInterceptor(opts.StreamInterceptors...))
	}
	if opts.Config.TLSConfig != nil {
		serverOptions = append(serverOptions, grpc.Creds(credentials.NewTLS(opts.Config.TLSConfig)))
	}

	grpcServer := grpc.NewServer(serverOptions...)
	for _, registration := range opts.Registrations {
		if err := registration.Fn(grpcServer); err != nil {
			return nil, fmt.Errorf("register grpc service %s: %w", registration.Name, err)
		}
	}

	return &Server{
		config:   opts.Config,
		logger:   log,
		server:   grpcServer,
		listener: opts.Listener,
	}, nil
}

// Start begins serving requests until the context is canceled or the server fails.
func (s *Server) Start(ctx context.Context) error {
	listener := s.listener
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", s.config.Address)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", s.config.Address, err)
		}
		s.listener = listener
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.Serve(listener); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("grpc server failed: %w", err)
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	}
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	timeout := s.config.ShutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-done:
		if s.listener != nil {
			_ = s.listener.Close()
		}
		return nil
	case <-shutdownCtx.Done():
		s.server.Stop()
		if s.listener != nil {
			_ = s.listener.Close()
		}
		return fmt.Errorf("grpc server shutdown timed out: %w", shutdownCtx.Err())
	}
}

// GRPCServer returns the underlying grpc.Server.
func (s *Server) GRPCServer() *grpc.Server {
	return s.server
}

// Run starts the gRPC server through the shared core runtime lifecycle.
func Run(ctx context.Context, srv *Server, opts Options) error {
	if srv == nil {
		return errors.New("grpc server is required")
	}
	log := opts.Logger
	if log == nil {
		log = srv.logger
	}
	if log == nil {
		return errors.New("logger is required")
	}
	name := opts.AppName
	if name == "" {
		name = "grpc"
	}

	lifecycleApp, err := coreapp.New(coreapp.Options{
		Name:                 name,
		Logger:               log,
		ShutdownTimeout:      srv.config.ShutdownTimeout,
		FeatureRegistrations: toCoreHooks(opts.StartupHooks),
		Runners: []coreapp.Runner{
			{
				Name: "grpc_server",
				Fn: func(ctx context.Context, runtime *coreapp.Runtime) error {
					return srv.Start(ctx)
				},
			},
		},
		ShutdownHooks: toCoreHooks(opts.ShutdownHooks),
	})
	if err != nil {
		return fmt.Errorf("create grpc lifecycle app: %w", err)
	}
	return lifecycleApp.Run(ctx)
}

func toCoreHooks(hooks []LifecycleHook) []coreapp.Hook {
	out := make([]coreapp.Hook, 0, len(hooks))
	for _, hook := range hooks {
		current := hook
		out = append(out, coreapp.Hook{
			Name: current.Name,
			Fn: func(ctx context.Context, runtime *coreapp.Runtime) error {
				return current.Fn(ctx)
			},
		})
	}
	return out
}
