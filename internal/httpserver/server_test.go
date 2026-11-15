package httpserver

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"datawark/internal/audit"
	"datawark/internal/config"
)

func TestServerUsesBoundedHTTPSettings(t *testing.T) {
	service := New("127.0.0.1:0")
	if service.server.ReadHeaderTimeout != ReadHeaderTimeout || service.server.IdleTimeout != IdleTimeout || service.server.MaxHeaderBytes != MaxHeaderBytes {
		t.Fatalf("server limits = %#v", service.server)
	}
}

func TestAuthenticationWritesOneEventAndFailsClosedOnAuditFailure(t *testing.T) {
	secret := "reader-key"
	authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER_KEY"}}, func(string) (string, bool) { return secret, true })
	if err != nil {
		t.Fatal(err)
	}
	var events bytes.Buffer
	authenticator.SetRecorder(audit.NewJSONLinesRecorder(&events))
	service := New("127.0.0.1:0")
	handler := service.requestID(authenticator.middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	request := httptest.NewRequest(http.MethodGet, "/data", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if strings.Count(events.String(), "\n") != 1 || !strings.Contains(events.String(), "\"operation\":\"authenticate\"") {
		t.Fatalf("events = %q", events.String())
	}
	authenticator.SetRecorder(audit.NewJSONLinesRecorder(failingAuditWriter{}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}

type failingAuditWriter struct{}

func (failingAuditWriter) Write([]byte) (int, error) {
	return 0, errors.New("audit writer unavailable")
}

func TestAuthenticatorBootstrapIsSecretSafe(t *testing.T) {
	secret := "distinctive-bootstrap-secret"
	keys := []config.APIKey{{Principal: "reader", KeyEnv: "READER_KEY"}, {Principal: "writer", KeyEnv: "WRITER_KEY"}}
	authenticator, err := NewAuthenticator(keys, func(name string) (string, bool) {
		return map[string]string{"READER_KEY": secret, "WRITER_KEY": "other-secret"}[name], true
	})
	if err != nil || authenticator == nil || len(authenticator.credentials) != 2 {
		t.Fatalf("NewAuthenticator() = %#v, %v", authenticator, err)
	}
	for _, test := range []struct {
		name   string
		keys   []config.APIKey
		lookup func(string) (string, bool)
	}{
		{"missing", keys[:1], func(string) (string, bool) { return "", false }},
		{"empty", keys[:1], func(string) (string, bool) { return "", true }},
		{"duplicate", keys, func(string) (string, bool) { return secret, true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewAuthenticator(test.keys, test.lookup)
			if err == nil || strings.Contains(err.Error(), secret) {
				t.Fatalf("unsafe bootstrap error: %v", err)
			}
		})
	}
}

func TestAuthenticatedRoutesFailClosedAndExposeTrustedPrincipal(t *testing.T) {
	secret := "hostile-secret-value"
	authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER_KEY"}}, func(string) (string, bool) { return secret, true })
	if err != nil {
		t.Fatal(err)
	}
	service := New("127.0.0.1:0", authenticator)
	protected := authenticator.middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := Principal(request.Context())
		if !ok || identity.Name() != "reader" || request.URL.Query().Get("principal") == identity.Name() {
			t.Fatalf("trusted identity = %#v, %v", identity, ok)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	for name, header := range map[string][]string{
		"missing": nil, "malformed": {"Basic " + secret}, "multiple": {"Bearer " + secret, "Bearer " + secret}, "unknown": {"Bearer unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/data", nil)
			request.Header["Authorization"] = header
			response := httptest.NewRecorder()
			protected.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" || strings.Contains(response.Body.String(), secret) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
	request := httptest.NewRequest(http.MethodGet, "/data?principal=writer", nil)
	request.Header.Set("Authorization", "bEaReR "+secret)
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authenticated status = %d", response.Code)
	}
	for _, path := range []string{"/api/v1/health", "/api/v1/ready", "/metrics"} {
		response = httptest.NewRecorder()
		service.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code == http.StatusUnauthorized {
			t.Fatalf("operational route %s required authentication", path)
		}
	}
	response = httptest.NewRecorder()
	service.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/unknown", nil))
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("anonymous unknown route = %d", response.Code)
	}
}

func TestAuthenticatorKeepsAPIKeyPrincipalsIsolated(t *testing.T) {
	authenticator, err := NewAuthenticator([]config.APIKey{
		{Principal: "reader", KeyEnv: "READER_KEY"},
		{Principal: "auditor", KeyEnv: "AUDITOR_KEY"},
	}, func(name string) (string, bool) {
		return map[string]string{"READER_KEY": "reader-key", "AUDITOR_KEY": "auditor-key"}[name], true
	})
	if err != nil {
		t.Fatal(err)
	}
	for credential, want := range map[string]string{"reader-key": "reader", "auditor-key": "auditor"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/query?principal=attacker", nil)
		request.Header.Set("Authorization", "Bearer "+credential)
		identity, ok := authenticator.Authenticate(request.Header)
		if !ok || identity.Name() != want {
			t.Fatalf("Authenticate(%q) = %q, %v; want %q", credential, identity.Name(), ok, want)
		}
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
