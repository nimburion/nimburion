// Package server provides HTTP server implementations with graceful startup and shutdown.
package server

import (
	"context"
	"strings"

	"github.com/nimburion/nimburion/pkg/config"
	eventbusfactory "github.com/nimburion/nimburion/pkg/eventbus/factory"
	"github.com/nimburion/nimburion/pkg/middleware/cors"
	"github.com/nimburion/nimburion/pkg/middleware/csrf"
	httpsignature "github.com/nimburion/nimburion/pkg/middleware/httpsignature"
	i18nmiddleware "github.com/nimburion/nimburion/pkg/middleware/i18n"
	"github.com/nimburion/nimburion/pkg/middleware/logging"
	"github.com/nimburion/nimburion/pkg/middleware/metrics"
	"github.com/nimburion/nimburion/pkg/middleware/recovery"
	"github.com/nimburion/nimburion/pkg/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/middleware/requestsize"
	"github.com/nimburion/nimburion/pkg/middleware/securityheaders"
	"github.com/nimburion/nimburion/pkg/middleware/session"
	timeoutmiddleware "github.com/nimburion/nimburion/pkg/middleware/timeout"
	"github.com/nimburion/nimburion/pkg/middleware/tracing"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/realtime/sse"
	"github.com/nimburion/nimburion/pkg/server/router"
	storememcached "github.com/nimburion/nimburion/pkg/store/memcached"
)

// PublicAPIServer wraps Server for application traffic.
// It provides the primary HTTP server for handling public API requests
// with a configured middleware stack for cross-cutting concerns.
type PublicAPIServer struct {
	*Server
	sessionStore session.Store
	sseManager   *sse.Manager
}

// NewPublicAPIServer creates a new PublicAPIServer instance.
// It configures the server with the HTTP configuration and applies
// the standard middleware stack (request ID, logging, recovery, metrics).
//
// The middleware stack is applied in the following order:
// 1. Request ID - generates/extracts request IDs for correlation
// 2. Logging - logs HTTP requests with structured data
// 3. Recovery - catches panics and returns 500 errors
// 4. Metrics - records Prometheus metrics for requests
//
// Additional middleware (auth, rate limiting) can be added per-route.
func NewPublicAPIServer(cfg config.HTTPConfig, r router.Router, log logger.Logger) *PublicAPIServer {
	defaults := config.DefaultConfig()
	return NewPublicAPIServerWithConfig(
		cfg,
		defaults.CORS,
		defaults.SecurityHeaders,
		defaults.Security,
		defaults.I18n,
		defaults.Session,
		defaults.CSRF,
		defaults.SSE,
		defaults.EventBus,
		defaults.Validation,
		defaults.Observability,
		r,
		log,
	)
}

// NewPublicAPIServerWithObservability creates a new PublicAPIServer with observability-aware middleware options.
func NewPublicAPIServerWithObservability(
	cfg config.HTTPConfig,
	obsCfg config.ObservabilityConfig,
	r router.Router,
	log logger.Logger,
) *PublicAPIServer {
	defaults := config.DefaultConfig()
	return NewPublicAPIServerWithConfig(
		cfg,
		defaults.CORS,
		defaults.SecurityHeaders,
		defaults.Security,
		defaults.I18n,
		defaults.Session,
		defaults.CSRF,
		defaults.SSE,
		defaults.EventBus,
		defaults.Validation,
		obsCfg,
		r,
		log,
	)
}

// NewPublicAPIServerWithConfig creates a new PublicAPIServer with CORS and observability-aware middleware options.
func NewPublicAPIServerWithConfig(
	cfg config.HTTPConfig,
	corsCfg config.CORSConfig,
	securityHeadersCfg config.SecurityHeadersConfig,
	securityCfg config.SecurityConfig,
	i18nCfg config.I18nConfig,
	sessionCfg config.SessionConfig,
	csrfCfg config.CSRFConfig,
	sseCfg config.SSEConfig,
	eventBusCfg config.EventBusConfig,
	validationCfg config.ValidationConfig,
	obsCfg config.ObservabilityConfig,
	r router.Router,
	log logger.Logger,
) *PublicAPIServer {
	effectiveLogger := logger.WrapAsync(log, logger.AsyncConfig{
		Enabled:      obsCfg.AsyncLogging.Enabled,
		QueueSize:    obsCfg.AsyncLogging.QueueSize,
		WorkerCount:  obsCfg.AsyncLogging.WorkerCount,
		DropWhenFull: obsCfg.AsyncLogging.DropWhenFull,
	})

	corsMiddlewareCfg := cors.Config{
		Enabled:                   corsCfg.Enabled,
		AllowAllOrigins:           corsCfg.AllowAllOrigins,
		AllowOrigins:              corsCfg.AllowOrigins,
		AllowMethods:              corsCfg.AllowMethods,
		AllowPrivateNetwork:       corsCfg.AllowPrivateNetwork,
		AllowHeaders:              corsCfg.AllowHeaders,
		ExposeHeaders:             corsCfg.ExposeHeaders,
		AllowCredentials:          corsCfg.AllowCredentials,
		MaxAge:                    corsCfg.MaxAge,
		AllowWildcard:             corsCfg.AllowWildcard,
		AllowBrowserExtensions:    corsCfg.AllowBrowserExtensions,
		CustomSchemas:             corsCfg.CustomSchemas,
		AllowWebSockets:           corsCfg.AllowWebSockets,
		AllowFiles:                corsCfg.AllowFiles,
		OptionsResponseStatusCode: corsCfg.OptionsResponseStatusCode,
	}
	securityHeadersMiddlewareCfg := securityheaders.Config{
		Enabled:                   securityHeadersCfg.Enabled,
		IsDevelopment:             securityHeadersCfg.IsDevelopment,
		AllowedHosts:              securityHeadersCfg.AllowedHosts,
		SSLRedirect:               securityHeadersCfg.SSLRedirect,
		SSLTemporaryRedirect:      securityHeadersCfg.SSLTemporaryRedirect,
		SSLHost:                   securityHeadersCfg.SSLHost,
		SSLProxyHeaders:           securityHeadersCfg.SSLProxyHeaders,
		DontRedirectIPV4Hostnames: securityHeadersCfg.DontRedirectIPV4Hostnames,
		STSSeconds:                securityHeadersCfg.STSSeconds,
		STSIncludeSubdomains:      securityHeadersCfg.STSIncludeSubdomains,
		STSPreload:                securityHeadersCfg.STSPreload,
		CustomFrameOptions:        securityHeadersCfg.CustomFrameOptions,
		ContentTypeNosniff:        securityHeadersCfg.ContentTypeNosniff,
		ContentSecurityPolicy:     securityHeadersCfg.ContentSecurityPolicy,
		ReferrerPolicy:            securityHeadersCfg.ReferrerPolicy,
		PermissionsPolicy:         securityHeadersCfg.PermissionsPolicy,
		IENoOpen:                  securityHeadersCfg.IENoOpen,
		XDNSPrefetchControl:       securityHeadersCfg.XDNSPrefetchControl,
		CrossOriginOpenerPolicy:   securityHeadersCfg.CrossOriginOpenerPolicy,
		CrossOriginResourcePolicy: securityHeadersCfg.CrossOriginResourcePolicy,
		CrossOriginEmbedderPolicy: securityHeadersCfg.CrossOriginEmbedderPolicy,
		CustomHeaders:             securityHeadersCfg.CustomHeaders,
	}
	sessionStore := createSessionStore(sessionCfg, effectiveLogger)
	sessionMiddlewareCfg := session.Config{
		Enabled:        sessionCfg.Enabled && sessionStore != nil,
		Store:          sessionStore,
		CookieName:     sessionCfg.CookieName,
		CookiePath:     sessionCfg.CookiePath,
		CookieDomain:   sessionCfg.CookieDomain,
		CookieSecure:   sessionCfg.CookieSecure,
		CookieHTTPOnly: sessionCfg.CookieHTTPOnly,
		CookieSameSite: sessionCfg.CookieSameSite,
		TTL:            sessionCfg.TTL,
		IdleTimeout:    sessionCfg.IdleTimeout,
		AutoCreate:     sessionCfg.AutoCreate,
	}
	csrfMiddlewareCfg := csrf.Config{
		Enabled:        csrfCfg.Enabled,
		HeaderName:     csrfCfg.HeaderName,
		CookieName:     csrfCfg.CookieName,
		CookiePath:     csrfCfg.CookiePath,
		CookieDomain:   csrfCfg.CookieDomain,
		CookieSecure:   csrfCfg.CookieSecure,
		CookieSameSite: csrfCfg.CookieSameSite,
		CookieTTL:      csrfCfg.CookieTTL,
		ExemptMethods:  csrfCfg.ExemptMethods,
		ExemptPaths:    csrfCfg.ExemptPaths,
	}
	i18nMiddlewareCfg := i18nmiddleware.Config{
		Enabled:              i18nCfg.Enabled,
		DefaultLocale:        i18nCfg.DefaultLocale,
		SupportedLocales:     i18nCfg.SupportedLocales,
		QueryParam:           i18nCfg.QueryParam,
		HeaderName:           i18nCfg.HeaderName,
		FallbackMode:         i18nCfg.FallbackMode,
		CatalogPath:          i18nCfg.CatalogPath,
		ExcludedPathPrefixes: append([]string{}, i18nmiddleware.DefaultConfig().ExcludedPathPrefixes...),
	}
	httpSignatureCfg := httpsignature.Config{
		Enabled:              securityCfg.HTTPSignature.Enabled,
		KeyIDHeader:          securityCfg.HTTPSignature.KeyIDHeader,
		TimestampHeader:      securityCfg.HTTPSignature.TimestampHeader,
		NonceHeader:          securityCfg.HTTPSignature.NonceHeader,
		SignatureHeader:      securityCfg.HTTPSignature.SignatureHeader,
		MaxClockSkew:         securityCfg.HTTPSignature.MaxClockSkew,
		NonceTTL:             securityCfg.HTTPSignature.NonceTTL,
		RequireNonce:         securityCfg.HTTPSignature.RequireNonce,
		ExcludedPathPrefixes: securityCfg.HTTPSignature.ExcludedPathPrefixes,
	}
	if len(securityCfg.HTTPSignature.StaticKeys) > 0 {
		staticProvider := httpsignature.StaticKeyProvider{}
		for keyID, secret := range securityCfg.HTTPSignature.StaticKeys {
			trimmedKeyID := strings.TrimSpace(keyID)
			trimmedSecret := strings.TrimSpace(secret)
			if trimmedKeyID == "" || trimmedSecret == "" {
				continue
			}
			staticProvider[trimmedKeyID] = trimmedSecret
		}
		if len(staticProvider) > 0 {
			httpSignatureCfg.KeyProvider = staticProvider
		}
	}

	loggingCfg := logging.Config{
		Enabled:              obsCfg.RequestLogging.Enabled,
		LogStart:             obsCfg.RequestLogging.LogStart,
		Output:               logging.Output(obsCfg.RequestLogging.Output),
		Fields:               obsCfg.RequestLogging.Fields,
		ExcludedPathPrefixes: obsCfg.RequestLogging.ExcludedPathPrefixes,
		PathPolicies:         make([]logging.PathPolicy, 0, len(obsCfg.RequestLogging.PathPolicies)),
	}
	for _, policy := range obsCfg.RequestLogging.PathPolicies {
		loggingCfg.PathPolicies = append(loggingCfg.PathPolicies, logging.PathPolicy{
			Prefix: policy.PathPrefix,
			Mode:   logging.Mode(policy.Mode),
		})
	}

	tracingCfg := tracing.Config{
		TracerName:           "http-server",
		ExcludedPathPrefixes: obsCfg.RequestTracing.ExcludedPathPrefixes,
		PathPolicies:         make([]tracing.PathPolicy, 0, len(obsCfg.RequestTracing.PathPolicies)),
	}
	for _, policy := range obsCfg.RequestTracing.PathPolicies {
		tracingCfg.PathPolicies = append(tracingCfg.PathPolicies, tracing.PathPolicy{
			Prefix: policy.PathPrefix,
			Mode:   tracing.Mode(policy.Mode),
		})
	}
	timeoutCfg := timeoutmiddleware.Config{
		Enabled:              obsCfg.RequestTimeout.Enabled,
		Default:              obsCfg.RequestTimeout.Default,
		ExcludedPathPrefixes: obsCfg.RequestTimeout.ExcludedPathPrefixes,
		PathPolicies:         make([]timeoutmiddleware.PathPolicy, 0, len(obsCfg.RequestTimeout.PathPolicies)),
	}
	for _, policy := range obsCfg.RequestTimeout.PathPolicies {
		timeoutCfg.PathPolicies = append(timeoutCfg.PathPolicies, timeoutmiddleware.PathPolicy{
			Prefix: policy.PathPrefix,
			Mode:   timeoutmiddleware.Mode(policy.Mode),
		})
	}

	// Apply standard middleware stack
	type middlewareEntry struct {
		name string
		fn   router.MiddlewareFunc
	}
	namedMiddlewares := []middlewareEntry{
		{name: "request_id", fn: requestid.RequestID()},
		{name: "http_signature", fn: httpsignature.Middleware(httpSignatureCfg)},
		{name: "security_headers", fn: securityheaders.Middleware(securityHeadersMiddlewareCfg)},
		{name: "session", fn: session.Middleware(sessionMiddlewareCfg)},
		{name: "csrf", fn: csrf.Middleware(csrfMiddlewareCfg)},
		{name: "cors", fn: cors.Middleware(corsMiddlewareCfg)},
		{name: "i18n", fn: i18nmiddleware.Middleware(i18nMiddlewareCfg)},
		{name: "logging", fn: logging.WithConfig(effectiveLogger, loggingCfg)},
		{name: "recovery", fn: recovery.Recovery(effectiveLogger)},
		{name: "metrics", fn: metrics.Metrics()},
	}
	if obsCfg.TracingEnabled && obsCfg.RequestTracing.Enabled {
		namedMiddlewares = append(namedMiddlewares, middlewareEntry{name: "tracing", fn: tracing.Tracing(tracingCfg)})
	}
	namedMiddlewares = append(namedMiddlewares, middlewareEntry{name: "timeout", fn: timeoutmiddleware.Middleware(timeoutCfg)})
	namedMiddlewares = append(namedMiddlewares, middlewareEntry{name: "request_size", fn: requestsize.Middleware(cfg.MaxRequestSize)})
	middlewareFuncs := make([]router.MiddlewareFunc, 0, len(namedMiddlewares))
	middlewareNames := make([]string, 0, len(namedMiddlewares))
	for _, entry := range namedMiddlewares {
		middlewareFuncs = append(middlewareFuncs, entry.fn)
		middlewareNames = append(middlewareNames, entry.name)
	}
	if len(middlewareNames) > 0 {
		effectiveLogger.Debug("active middleware stack", "middlewares", strings.Join(middlewareNames, ", "))
	}
	r.Use(middlewareFuncs...)

	sseManager := createSSEManager(sseCfg, eventBusCfg, validationCfg.Kafka, effectiveLogger)
	if sseManager != nil {
		handler, err := sse.NewHandler(sse.HandlerConfig{
			Manager:               sseManager,
			ChannelQueryParam:     sseCfg.ChannelQueryParam,
			TenantQueryParam:      sseCfg.TenantQueryParam,
			SubjectQueryParam:     sseCfg.SubjectQueryParam,
			LastEventIDQueryParam: sseCfg.LastEventIDQueryParam,
		})
		if err != nil {
			effectiveLogger.Error("failed to initialize sse handler; endpoint disabled", "error", err)
		} else {
			endpoint := strings.TrimSpace(sseCfg.Endpoint)
			if endpoint == "" {
				endpoint = config.DefaultConfig().SSE.Endpoint
			}
			r.GET(endpoint, handler.Stream())
		}
	}

	// Create server config from HTTP config
	serverCfg := Config{
		Port:         cfg.Port,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Create base server
	baseServer := NewServer(serverCfg, r, effectiveLogger)

	return &PublicAPIServer{
		Server:       baseServer,
		sessionStore: sessionStore,
		sseManager:   sseManager,
	}
}

// Start starts the public API server.
// It delegates to the underlying Server's Start method.
func (s *PublicAPIServer) Start(ctx context.Context) error {
	return s.Server.Start(ctx)
}

// Shutdown gracefully shuts down the public API server.
// It delegates to the underlying Server's Shutdown method.
func (s *PublicAPIServer) Shutdown(ctx context.Context) error {
	if err := s.Server.Shutdown(ctx); err != nil {
		return err
	}
	if s.sseManager != nil {
		if err := s.sseManager.Close(); err != nil {
			return err
		}
	}
	if s.sessionStore != nil {
		return s.sessionStore.Close()
	}
	return nil
}

// Router returns the public API server's router instance
func (s *PublicAPIServer) Router() *router.Router {
	return &s.router
}

func createSessionStore(cfg config.SessionConfig, log logger.Logger) session.Store {
	if !cfg.Enabled {
		return nil
	}

	store := strings.ToLower(strings.TrimSpace(cfg.Store))
	switch store {
	case "", "inmemory":
		return session.NewInMemoryStore()
	case "redis":
		redisStore, err := session.NewRedisStore(session.RedisConfig{
			URL:              cfg.Redis.URL,
			MaxConns:         cfg.Redis.MaxConns,
			OperationTimeout: cfg.Redis.OperationTimeout,
			Prefix:           cfg.Redis.Prefix,
		})
		if err != nil {
			log.Error("failed to initialize redis session store, disabling session middleware", "error", err)
			return nil
		}
		return redisStore
	case "memcached":
		memcachedClient, err := storememcached.NewMemcachedAdapter(cfg.Memcached.Addresses, cfg.Memcached.Timeout)
		if err != nil {
			log.Error("failed to initialize memcached session client, disabling session middleware", "error", err)
			return nil
		}
		memcachedStore, err := session.NewMemcachedStore(memcachedClient, cfg.Memcached.Prefix)
		if err != nil {
			log.Error("failed to initialize memcached session store, disabling session middleware", "error", err)
			return nil
		}
		return memcachedStore
	default:
		log.Warn("unknown session store configured; session middleware disabled", "store", cfg.Store)
		return nil
	}
}

func createSSEManager(
	sseCfg config.SSEConfig,
	eventBusCfg config.EventBusConfig,
	kafkaValidationCfg config.KafkaValidationConfig,
	log logger.Logger,
) *sse.Manager {
	if !sseCfg.Enabled {
		return nil
	}

	var store sse.Store
	switch strings.ToLower(strings.TrimSpace(sseCfg.Store)) {
	case "", "inmemory":
		store = sse.NewInMemoryStore(sseCfg.ReplayLimit)
	case "redis":
		redisStore, err := sse.NewRedisStore(sse.RedisStoreConfig{
			URL:              sseCfg.Redis.URL,
			Prefix:           sseCfg.Redis.HistoryPrefix,
			MaxSize:          int64(sseCfg.ReplayLimit),
			OperationTimeout: sseCfg.Redis.OperationTimeout,
			MaxConns:         sseCfg.Redis.MaxConns,
		})
		if err != nil {
			log.Error("failed to initialize sse redis store; sse disabled", "error", err)
			return nil
		}
		store = redisStore
	default:
		log.Warn("unknown sse.store configured; sse disabled", "store", sseCfg.Store)
		return nil
	}

	var bus sse.Bus
	switch strings.ToLower(strings.TrimSpace(sseCfg.Bus)) {
	case "", "none":
		bus = nil
	case "inmemory":
		bus = sse.NewInMemoryBus()
	case "redis":
		redisBus, err := sse.NewRedisBus(sse.RedisBusConfig{
			URL:              sseCfg.Redis.URL,
			Prefix:           sseCfg.Redis.PubSubPrefix,
			OperationTimeout: sseCfg.Redis.OperationTimeout,
			MaxConns:         sseCfg.Redis.MaxConns,
		})
		if err != nil {
			log.Error("failed to initialize sse redis bus; sse disabled", "error", err)
			_ = store.Close()
			return nil
		}
		bus = redisBus
	case "eventbus":
		frameworkBus, err := eventbusfactory.NewEventBusAdapterWithValidation(eventBusCfg, kafkaValidationCfg, log)
		if err != nil {
			log.Error("failed to initialize framework eventbus for sse; sse disabled", "error", err)
			_ = store.Close()
			return nil
		}
		eventBusBridge, err := sse.NewEventBusAdapter(frameworkBus, sse.EventBusConfig{
			TopicPrefix:      sseCfg.EventBus.TopicPrefix,
			OperationTimeout: sseCfg.EventBus.OperationTimeout,
		})
		if err != nil {
			log.Error("failed to initialize sse eventbus bridge; sse disabled", "error", err)
			_ = frameworkBus.Close()
			_ = store.Close()
			return nil
		}
		bus = eventBusBridge
	default:
		log.Warn("unknown sse.bus configured; sse disabled", "bus", sseCfg.Bus)
		_ = store.Close()
		return nil
	}

	return sse.NewManager(sse.ManagerConfig{
		InstanceID:         "public-api",
		MaxConnections:     sseCfg.MaxConnections,
		ClientBuffer:       sseCfg.ClientBuffer,
		ReplayLimit:        sseCfg.ReplayLimit,
		DropOnBackpressure: sseCfg.DropOnBackpressure,
		HeartbeatInterval:  sseCfg.HeartbeatInterval,
		DefaultRetryMS:     sseCfg.DefaultRetryMS,
	}, store, bus)
}
