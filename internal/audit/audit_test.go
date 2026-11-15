package audit

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"datawark/internal/execution"
	"datawark/internal/query"
)

func event() Event {
	return Event{Version: VersionV1Alpha1, Timestamp: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC), RequestID: "request-1", Transport: TransportREST, Operation: OperationAuthenticate, Outcome: OutcomeSuccess, Reason: ReasonNone}
}

func TestEventValidation(t *testing.T) {
	for name, mutate := range map[string]func(*Event){
		"version":        func(e *Event) { e.Version = "v1" },
		"transport":      func(e *Event) { e.Transport = "stdio" },
		"operation":      func(e *Event) { e.Operation = "sql" },
		"success reason": func(e *Event) { e.Reason = ReasonPolicy },
		"failure reason": func(e *Event) { e.Outcome = OutcomeDenied },
	} {
		t.Run(name, func(t *testing.T) {
			got := event()
			mutate(&got)
			if got.Validate() == nil {
				t.Fatal("Validate() succeeded")
			}
		})
	}
}

func TestJSONLinesRecorderUsesOnlyDocumentedFields(t *testing.T) {
	var output bytes.Buffer
	if err := NewJSONLinesRecorder(&output).Record(event()); err != nil {
		t.Fatal(err)
	}
	want := "{\"version\":\"v1alpha1\",\"timestamp\":\"2026-11-15T12:00:00Z\",\"requestId\":\"request-1\",\"transport\":\"rest\",\"operation\":\"authenticate\",\"outcome\":\"success\",\"reason\":\"none\",\"durationMs\":0}\n"
	if output.String() != want {
		t.Fatalf("event = %q, want %q", output.String(), want)
	}
}

func TestJSONLinesRecorderSerializesConcurrentEvents(t *testing.T) {
	var output bytes.Buffer
	recorder := NewJSONLinesRecorder(&output)
	var group sync.WaitGroup
	for i := 0; i < 100; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := recorder.Record(event()); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if lines := strings.Count(output.String(), "\n"); lines != 100 {
		t.Fatalf("lines = %d", lines)
	}
}

type failingWriter struct{ value string }

func (writer failingWriter) Write([]byte) (int, error) { return 0, errors.New(writer.value) }

func TestRecorderFailureDoesNotExposeWriterError(t *testing.T) {
	hostile := []string{
		"api-key-should-not-appear", "sha256:digest-should-not-appear", "Authorization: Bearer hidden",
		"postgres://reader:hostile-password@example.test/db", "SELECT secret FROM hidden_table", "bound-argument",
		"returned-row", "request-body", "hidden_resource", "raw database error",
	}
	for _, value := range hostile {
		err := NewJSONLinesRecorder(failingWriter{value: value}).Record(event())
		if err == nil || strings.Contains(err.Error(), value) {
			t.Fatalf("unsafe error for %q = %v", value, err)
		}
	}
}

func TestClassifyUsesBoundedReasons(t *testing.T) {
	for _, test := range []struct {
		err     error
		outcome Outcome
		reason  Reason
	}{
		{nil, OutcomeSuccess, ReasonNone}, {&query.ValidationError{}, OutcomeInvalid, ReasonValidation},
		{execution.Error{Category: execution.ErrorUnavailable}, OutcomeUnavailable, ReasonUnavailable},
		{execution.Error{Category: execution.ErrorCapacity}, OutcomeDenied, ReasonGuardrail},
		{execution.Error{Category: execution.ErrorFailed}, OutcomeFailed, ReasonExecution},
		{errors.New("policy: query is not authorized"), OutcomeDenied, ReasonPolicy},
		{errors.New("schema: referenced metadata is unavailable"), OutcomeDenied, ReasonMetadata},
		{errors.New("runtime: datasource is unavailable"), OutcomeUnavailable, ReasonUnavailable},
		{errors.New("compiler: invalid table identifier"), OutcomeInvalid, ReasonValidation},
		{errors.New("SELECT secret FROM hidden"), OutcomeFailed, ReasonInternal},
	} {
		if outcome, reason := Classify(test.err); outcome != test.outcome || reason != test.reason {
			t.Fatalf("Classify(%v) = %s/%s", test.err, outcome, reason)
		}
	}
}
