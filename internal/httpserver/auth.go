package httpserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"datawark/internal/audit"
	"datawark/internal/config"
	"datawark/internal/principal"
)

type principalContextKey struct{}

// Principal returns the trusted identity established by authentication.
func Principal(ctx context.Context) (principal.Trusted, bool) {
	identity, ok := ctx.Value(principalContextKey{}).(principal.Trusted)
	return identity, ok && identity.Valid()
}

type credential struct {
	digest    [sha256.Size]byte
	principal principal.Trusted
}

// Authenticator keeps only SHA-256 digests after bootstrap.
type Authenticator struct {
	credentials []credential
	recorder    audit.Recorder
}

// SetRecorder configures the synchronous HTTP audit boundary.
func (authenticator *Authenticator) SetRecorder(recorder audit.Recorder) {
	authenticator.recorder = recorder
}

// Recorder returns the audit boundary shared by authenticated REST operations.
func (authenticator *Authenticator) Recorder() audit.Recorder {
	if authenticator == nil {
		return nil
	}
	return authenticator.recorder
}

// NewAuthenticator resolves all configured secrets before the listener is
// created. Errors deliberately identify only configuration categories.
func NewAuthenticator(keys []config.APIKey, lookup func(string) (string, bool)) (*Authenticator, error) {
	if lookup == nil {
		return nil, errors.New("API key authentication configuration is invalid")
	}
	authenticator := &Authenticator{credentials: make([]credential, 0, len(keys))}
	for _, key := range keys {
		value, found := lookup(key.KeyEnv)
		if !found || strings.TrimSpace(value) == "" {
			return nil, errors.New("API key environment variable is unavailable")
		}
		identity, err := principal.New(key.Principal)
		if err != nil {
			return nil, errors.New("API key authentication configuration is invalid")
		}
		digest := sha256.Sum256([]byte(value))
		for _, existing := range authenticator.credentials {
			if subtle.ConstantTimeCompare(existing.digest[:], digest[:]) == 1 {
				return nil, errors.New("API key credentials must be distinct")
			}
		}
		authenticator.credentials = append(authenticator.credentials, credential{digest: digest, principal: identity})
	}
	if len(authenticator.credentials) == 0 {
		return nil, errors.New("API key authentication configuration is invalid")
	}
	return authenticator, nil
}

// Authenticate validates one Bearer credential and scans every configured
// digest before returning its mapped principal.
func (authenticator *Authenticator) Authenticate(headers http.Header) (principal.Trusted, bool) {
	if authenticator == nil {
		return principal.Trusted{}, false
	}
	values := headers.Values("Authorization")
	if len(values) != 1 {
		return principal.Trusted{}, false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return principal.Trusted{}, false
	}
	digest := sha256.Sum256([]byte(parts[1]))
	var matched principal.Trusted
	found := 0
	for _, credential := range authenticator.credentials {
		match := subtle.ConstantTimeCompare(credential.digest[:], digest[:])
		if match == 1 {
			matched = credential.principal
		}
		found |= match
	}
	return matched, found == 1
}

func (authenticator *Authenticator) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if operationalPath(request.URL.Path) {
			next.ServeHTTP(writer, request)
			return
		}
		started := time.Now()
		identity, ok := authenticator.Authenticate(request.Header)
		event := audit.Event{Version: audit.VersionV1Alpha1, Timestamp: started.UTC(), RequestID: RequestID(request.Context()), Transport: authenticationTransport(request.URL.Path), Operation: audit.OperationAuthenticate, DurationMS: time.Since(started).Milliseconds()}
		if !ok {
			event.Outcome, event.Reason = audit.OutcomeDenied, audit.ReasonAuthentication
			if !authenticator.record(event) {
				unavailable(writer, request)
				return
			}
			unauthorized(writer, request)
			return
		}
		event.Principal, event.Outcome, event.Reason = identity.Name(), audit.OutcomeSuccess, audit.ReasonNone
		if !authenticator.record(event) {
			unavailable(writer, request)
			return
		}
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), principalContextKey{}, identity)))
	})
}

func authenticationTransport(path string) audit.Transport {
	if path == "/mcp" {
		return audit.TransportMCPHTTP
	}
	return audit.TransportREST
}

func (authenticator *Authenticator) record(event audit.Event) bool {
	return authenticator.recorder == nil || authenticator.recorder.Record(event) == nil
}

func operationalPath(path string) bool {
	return path == "/api/v1/health" || path == "/api/v1/ready" || path == "/metrics"
}

func unauthorized(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("WWW-Authenticate", "Bearer")
	writeError(writer, request, http.StatusUnauthorized, "unauthenticated", "authentication is required")
}

func unavailable(writer http.ResponseWriter, request *http.Request) {
	writeError(writer, request, http.StatusServiceUnavailable, "unavailable", "service is unavailable")
}
