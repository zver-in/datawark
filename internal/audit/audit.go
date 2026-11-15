// Package audit defines the secret-safe HTTP security event boundary.
package audit

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"datawark/internal/execution"
	"datawark/internal/query"
)

const VersionV1Alpha1 = "v1alpha1"

type Transport string

const (
	TransportREST    Transport = "rest"
	TransportMCPHTTP Transport = "mcp_http"
)

type Operation string

const (
	OperationAuthenticate Operation = "authenticate"
	OperationSchemaList   Operation = "schema_list"
	OperationSchemaGet    Operation = "schema_get"
	OperationQuery        Operation = "query"
	OperationExplain      Operation = "explain"
)

type Outcome string

const (
	OutcomeSuccess     Outcome = "success"
	OutcomeDenied      Outcome = "denied"
	OutcomeInvalid     Outcome = "invalid"
	OutcomeUnavailable Outcome = "unavailable"
	OutcomeFailed      Outcome = "failed"
)

type Reason string

const (
	ReasonNone           Reason = "none"
	ReasonAuthentication Reason = "authentication"
	ReasonPolicy         Reason = "policy"
	ReasonValidation     Reason = "validation"
	ReasonMetadata       Reason = "metadata"
	ReasonGuardrail      Reason = "guardrail"
	ReasonExecution      Reason = "execution"
	ReasonUnavailable    Reason = "unavailable"
	ReasonInternal       Reason = "internal"
)

// Event is deliberately limited to trusted identifiers and fixed categories.
// It has no field that can carry a credential, request content, database data,
// resource name, or raw error.
type Event struct {
	Version    string    `json:"version"`
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"requestId"`
	Transport  Transport `json:"transport"`
	Operation  Operation `json:"operation"`
	Principal  string    `json:"principal,omitempty"`
	Outcome    Outcome   `json:"outcome"`
	Reason     Reason    `json:"reason"`
	DurationMS int64     `json:"durationMs"`
}

func (event Event) Validate() error {
	if event.Version != VersionV1Alpha1 || event.Timestamp.IsZero() || event.RequestID == "" || event.DurationMS < 0 {
		return errors.New("audit event is invalid")
	}
	if !validTransport(event.Transport) || !validOperation(event.Operation) || !validOutcome(event.Outcome) || !validReason(event.Reason) {
		return errors.New("audit event is invalid")
	}
	if event.Operation == OperationAuthenticate && event.Transport != TransportREST && event.Transport != TransportMCPHTTP {
		return errors.New("audit event is invalid")
	}
	if event.Outcome == OutcomeSuccess && event.Reason != ReasonNone {
		return errors.New("audit event is invalid")
	}
	if event.Outcome != OutcomeSuccess && event.Reason == ReasonNone {
		return errors.New("audit event is invalid")
	}
	return nil
}

func validTransport(value Transport) bool { return value == TransportREST || value == TransportMCPHTTP }
func validOperation(value Operation) bool {
	return value == OperationAuthenticate || value == OperationSchemaList || value == OperationSchemaGet || value == OperationQuery || value == OperationExplain
}
func validOutcome(value Outcome) bool {
	return value == OutcomeSuccess || value == OutcomeDenied || value == OutcomeInvalid || value == OutcomeUnavailable || value == OutcomeFailed
}
func validReason(value Reason) bool {
	return value == ReasonNone || value == ReasonAuthentication || value == ReasonPolicy || value == ReasonValidation || value == ReasonMetadata || value == ReasonGuardrail || value == ReasonExecution || value == ReasonUnavailable || value == ReasonInternal
}

// Recorder performs the synchronous handoff required before an HTTP response
// is committed.
type Recorder interface{ Record(Event) error }

type JSONLinesRecorder struct {
	writer io.Writer
	mu     sync.Mutex
}

// Classify maps internal failures to the bounded audit vocabulary. It never
// preserves an error message or other untrusted detail.
func Classify(err error) (Outcome, Reason) {
	if err == nil {
		return OutcomeSuccess, ReasonNone
	}
	var validation *query.ValidationError
	if errors.As(err, &validation) {
		return OutcomeInvalid, ReasonValidation
	}
	var executionError execution.Error
	if errors.As(err, &executionError) {
		switch executionError.Category {
		case execution.ErrorUnavailable:
			return OutcomeUnavailable, ReasonUnavailable
		case execution.ErrorTimeout, execution.ErrorCapacity, execution.ErrorRowLimit, execution.ErrorResultSize:
			return OutcomeDenied, ReasonGuardrail
		default:
			return OutcomeFailed, ReasonExecution
		}
	}
	// These packages deliberately expose only stable, secret-free boundary
	// errors. Match their prefixes solely to select an audit category; callers
	// never serialize the source error.
	switch {
	case strings.HasPrefix(err.Error(), "policy:"):
		return OutcomeDenied, ReasonPolicy
	case strings.HasPrefix(err.Error(), "schema:"):
		return OutcomeDenied, ReasonMetadata
	case strings.HasPrefix(err.Error(), "runtime:"):
		return OutcomeUnavailable, ReasonUnavailable
	case strings.HasPrefix(err.Error(), "compiler:"), strings.HasPrefix(err.Error(), "query plan:"):
		return OutcomeInvalid, ReasonValidation
	}
	return OutcomeFailed, ReasonInternal
}

func NewJSONLinesRecorder(writer io.Writer) *JSONLinesRecorder {
	return &JSONLinesRecorder{writer: writer}
}

func (recorder *JSONLinesRecorder) Record(event Event) error {
	if recorder == nil || recorder.writer == nil || event.Validate() != nil {
		return errors.New("audit event could not be recorded")
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return errors.New("audit event could not be recorded")
	}
	encoded = append(encoded, '\n')
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if _, err := recorder.writer.Write(encoded); err != nil {
		return errors.New("audit event could not be recorded")
	}
	return nil
}
