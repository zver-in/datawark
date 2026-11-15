package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServerUsesBoundedHTTPSettings(t *testing.T) {
	service := New("127.0.0.1:0")
	if service.server.ReadHeaderTimeout != ReadHeaderTimeout || service.server.IdleTimeout != IdleTimeout || service.server.MaxHeaderBytes != MaxHeaderBytes {
		t.Fatalf("server limits = %#v", service.server)
	}
}

func TestRequestID(t *testing.T) {
	service := New("127.0.0.1:0")
	for name, supplied := range map[string]string{
		"valid":     "request-123._ok",
		"missing":   "",
		"oversized": strings.Repeat("a", 129),
		"unsafe":    "unsafe value",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			if supplied != "" {
				request.Header.Set(RequestIDHeader, supplied)
			}
			response := httptest.NewRecorder()
			service.Handler().ServeHTTP(response, request)
			got := response.Header().Get(RequestIDHeader)
			if !validRequestID(got) || (name == "valid" && got != supplied) || (name != "valid" && got == supplied) || strings.Contains(got, "unsafe") {
				t.Fatalf("request ID = %q for %s", got, name)
			}
		})
	}
}

func TestRequestIDIsPropagatedToContext(t *testing.T) {
	service := New("127.0.0.1:0")
	handler := service.requestID(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := RequestID(request.Context()); got != "request-123" {
			t.Fatalf("context request ID = %q", got)
		}
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(RequestIDHeader, "request-123")
	handler.ServeHTTP(httptest.NewRecorder(), request)
}

func TestProbesReflectReadiness(t *testing.T) {
	service := New("127.0.0.1:0")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("initial readiness = %d", response.Code)
	}
	service.ready.Store(true)
	response = httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("ready status = %d", response.Code)
	}
	context, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = service.Shutdown(context)
	for _, path := range []string{"/api/v1/health", "/api/v1/ready"} {
		response = httptest.NewRecorder()
		service.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		want := http.StatusOK
		if path == "/api/v1/ready" {
			want = http.StatusServiceUnavailable
		}
		if response.Code != want {
			t.Fatalf("%s = %d, want %d", path, response.Code, want)
		}
	}
}

func TestShutdownForcesExpiredInFlightRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	service := New(listener.Addr().String())
	original := service.server.Handler
	started := make(chan struct{})
	release := make(chan struct{})
	service.server.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/block" {
			original.ServeHTTP(writer, request)
			return
		}
		close(started)
		<-release
	})
	served := make(chan error, 1)
	go func() { served <- service.Serve(listener) }()
	requestDone := make(chan struct{})
	go func() {
		_, _ = http.Get("http://" + listener.Addr().String() + "/block")
		close(requestDone)
	}()
	<-started
	deadline, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- service.Shutdown(deadline) }()
	for service.ready.Load() {
		time.Sleep(time.Millisecond)
	}
	response := httptest.NewRecorder()
	original.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("withdrawn readiness = %d", response.Code)
	}
	if err := <-shutdownDone; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() error = %v", err)
	}
	<-requestDone
	close(release)
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}
