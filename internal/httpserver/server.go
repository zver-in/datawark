// Package httpserver owns the Gateway HTTP listener and its transport-neutral routes.
package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"datawark/internal/audit"
	"datawark/internal/mcpserver"
	"datawark/internal/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	RequestIDHeader   = "X-Request-ID"
	ReadHeaderTimeout = 5 * time.Second
	IdleTimeout       = 60 * time.Second
	WriteTimeout      = 30 * time.Second
	MaxHeaderBytes    = 16 << 10
	ShutdownTimeout   = 10 * time.Second
)

type requestIDContextKey struct{}

// RequestID returns the effective boundary request identifier.
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

// Server owns one HTTP server and its readiness state.
type Server struct {
	server   *http.Server
	ready    atomic.Bool
	logger   *slog.Logger
	metrics  requestMetrics
	registry *prometheus.Registry
}

type requestMetrics interface {
	Record(context.Context, int, time.Duration) error
}

func New(address string, authenticators ...*Authenticator) *Server {
	return newServer(address, nil, nil, authenticators...)
}

// NewWithGateway mounts the authenticated REST data API on the shared listener.
func NewWithGateway(address string, authenticator *Authenticator, bind GatewayBinder) *Server {
	var recorder audit.Recorder
	if authenticator != nil {
		recorder = authenticator.Recorder()
	}
	return newServer(address, bind, recorder, authenticator)
}

func newServer(address string, bind GatewayBinder, recorder audit.Recorder, authenticators ...*Authenticator) *Server {
	metrics := telemetry.NewMetrics()
	service := &Server{logger: telemetry.NewLogger(), metrics: metrics, registry: metrics.Registry}
	router := chi.NewRouter()
	router.Get("/api/v1/health", service.health)
	router.Get("/api/v1/ready", service.readiness)
	router.Handle("/metrics", promhttp.HandlerFor(service.registry, promhttp.HandlerOpts{}))
	if bind != nil {
		newRESTAPI(bind, recorder).routes(router)
		router.Handle("/mcp", mcpHTTPHandler(bind, recorder, service.logger))
	}
	var handler http.Handler = router
	if len(authenticators) == 1 && authenticators[0] != nil {
		handler = authenticators[0].middleware(handler)
	}
	// Request metadata must be installed before observation so the outer
	// completion boundary sees annotations refined by authentication and routes.
	handler = service.observe(handler)
	handler = service.requestID(handler)
	service.server = &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: ReadHeaderTimeout,
		IdleTimeout:       IdleTimeout,
		WriteTimeout:      WriteTimeout,
		MaxHeaderBytes:    MaxHeaderBytes,
	}
	return service
}

func mcpHTTPHandler(bind GatewayBinder, recorder audit.Recorder, operationalLogger *slog.Logger) http.Handler {
	logger := slog.New(safeMCPLogHandler{base: operationalLogger.Handler()})
	return mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		identity, ok := Principal(request.Context())
		if !ok || bind == nil {
			return nil
		}
		gateway, err := bind(identity)
		if err != nil || gateway == nil {
			return nil
		}
		return mcpserver.New(mcpserver.NewAuditedGateway(gateway, recorder, RequestID(request.Context()), identity.Name()))
	}, &mcp.StreamableHTTPOptions{Stateless: true, Logger: logger})
}

// safeMCPLogHandler preserves the existence and severity of SDK diagnostics
// without allowing protocol-derived messages or attributes into operational
// logs. MCP responses never use this logger.
type safeMCPLogHandler struct{ base slog.Handler }

func (handler safeMCPLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return handler.base.Enabled(ctx, level)
}

func (handler safeMCPLogHandler) Handle(ctx context.Context, record slog.Record) error {
	return handler.base.Handle(ctx, slog.NewRecord(record.Time, record.Level, "mcp streamable HTTP diagnostic", record.PC))
}

func (handler safeMCPLogHandler) WithAttrs([]slog.Attr) slog.Handler { return handler }
func (handler safeMCPLogHandler) WithGroup(string) slog.Handler      { return handler }

func (service *Server) Handler() http.Handler { return service.server.Handler }

// Serve begins processing only after the caller has completed runtime bootstrap.
func (service *Server) Serve(listener net.Listener) error {
	service.ready.Store(true)
	service.logger.Info("http server started", "event", "startup")
	err := service.server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown first withdraws readiness, then bounds graceful in-flight completion.
func (service *Server) Shutdown(ctx context.Context) error {
	service.ready.Store(false)
	service.logger.Info("http server readiness withdrawn", "event", "readiness_withdrawn")
	if err := service.server.Shutdown(ctx); err != nil {
		_ = service.server.Close()
		return err
	}
	service.logger.Info("http server stopped", "event", "shutdown")
	return nil
}

func (service *Server) health(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func (service *Server) readiness(writer http.ResponseWriter, _ *http.Request) {
	if !service.ready.Load() {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (service *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id := request.Header.Get(RequestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}
		writer.Header().Set(RequestIDHeader, id)
		transport, operation := requestTelemetry(request.URL.Path)
		ctx := context.WithValue(request.Context(), requestIDContextKey{}, id)
		next.ServeHTTP(writer, request.WithContext(telemetry.WithRequest(ctx, transport, operation)))
	})
}

func requestTelemetry(path string) (string, string) {
	switch path {
	case "/api/v1/query":
		return telemetry.TransportREST, "query"
	case "/api/v1/query/explain":
		return telemetry.TransportREST, "explain"
	case "/api/v1/schema":
		return telemetry.TransportREST, "schema_list"
	case "/mcp":
		return telemetry.TransportMCP, "mcp"
	case "/metrics":
		return telemetry.TransportOps, "metrics"
	case "/api/v1/health":
		return telemetry.TransportOps, "health"
	case "/api/v1/ready":
		return telemetry.TransportOps, "ready"
	default:
		return telemetry.Other, telemetry.Other
	}
}

type responseCapture struct {
	http.ResponseWriter
	status, bytes int
}

func (writer *responseCapture) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
	writer.ResponseWriter.WriteHeader(status)
}
func (writer *responseCapture) Write(data []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	n, err := writer.ResponseWriter.Write(data)
	writer.bytes += n
	return n, err
}

func (service *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		captured := &responseCapture{ResponseWriter: writer}
		next.ServeHTTP(captured, request)
		if captured.status == 0 {
			captured.status = http.StatusOK
		}
		elapsed := time.Since(started)
		_ = service.metrics.Record(request.Context(), captured.status, elapsed)
		transport, operation, auth, policy, execution := telemetry.Snapshot(request.Context())
		service.logger.Info("http request complete", "request_id", RequestID(request.Context()), "transport", transport, "operation", operation, "outcome", telemetry.Outcome(request.Context(), captured.status), "authentication", auth, "policy", policy, "execution", execution, "status_class", telemetry.StatusClass(captured.status), "status", captured.status, "bytes", captured.bytes, "duration_ms", elapsed.Milliseconds())
	})
}

func validRequestID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-') {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic("request ID randomness unavailable")
	}
	return hex.EncodeToString(bytes[:])
}
