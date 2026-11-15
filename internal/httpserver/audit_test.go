package httpserver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"datawark/internal/audit"
)

func TestAuditedOperationHandsOffBeforeSuccess(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	request = request.WithContext(contextWithRequestID(request.Context(), "request-1"))
	var output bytes.Buffer
	if err := AuditedOperation(request, audit.NewJSONLinesRecorder(&output), audit.OperationQuery, "reader", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "\n") != 1 || !strings.Contains(output.String(), "\"operation\":\"query\"") {
		t.Fatalf("event = %q", output.String())
	}
	if err := AuditedOperation(request, audit.NewJSONLinesRecorder(failingAuditWriter{}), audit.OperationQuery, "reader", func() error { return nil }); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func contextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}
