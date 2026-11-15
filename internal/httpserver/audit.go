package httpserver

import (
	"errors"
	"net/http"
	"time"

	"datawark/internal/audit"
)

var ErrAuditUnavailable = errors.New("audit handoff unavailable")

// AuditedOperation runs a data operation before its response is committed and
// records exactly one terminal event. Callers must write a success response
// only after it returns nil, so a failed audit handoff discards that result.
func AuditedOperation(request *http.Request, recorder audit.Recorder, operation audit.Operation, principal string, run func() error) error {
	started := time.Now()
	err := run()
	outcome, reason := audit.Classify(err)
	event := audit.Event{Version: audit.VersionV1Alpha1, Timestamp: started.UTC(), RequestID: RequestID(request.Context()), Transport: audit.TransportREST, Operation: operation, Principal: principal, Outcome: outcome, Reason: reason, DurationMS: time.Since(started).Milliseconds()}
	if recorder != nil && recorder.Record(event) != nil {
		return ErrAuditUnavailable
	}
	return err
}
