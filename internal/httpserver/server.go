// Package httpserver owns the Gateway HTTP listener and its transport-neutral routes.
package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
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
	server *http.Server
	ready  atomic.Bool
}

func New(address string, authenticators ...*Authenticator) *Server {
	service := &Server{}
	router := chi.NewRouter()
	router.Get("/api/v1/health", service.health)
	router.Get("/api/v1/ready", service.readiness)
	var handler http.Handler = router
	if len(authenticators) == 1 && authenticators[0] != nil {
		handler = authenticators[0].middleware(handler)
	}
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

func (service *Server) Handler() http.Handler { return service.server.Handler }

// Serve begins processing only after the caller has completed runtime bootstrap.
func (service *Server) Serve(listener net.Listener) error {
	service.ready.Store(true)
	err := service.server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown first withdraws readiness, then bounds graceful in-flight completion.
func (service *Server) Shutdown(ctx context.Context) error {
	service.ready.Store(false)
	if err := service.server.Shutdown(ctx); err != nil {
		_ = service.server.Close()
		return err
	}
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
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, id)))
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
