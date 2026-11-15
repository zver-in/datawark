package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"datawark/internal/audit"
	"datawark/internal/execution"
	"datawark/internal/policy"
	"datawark/internal/principal"
	"datawark/internal/query"
	"datawark/internal/schema"
	"datawark/internal/telemetry"
	"github.com/go-chi/chi/v5"
)

const MaxPlanBodyBytes int64 = 1 << 20

// Gateway is the deliberately small, principal-bound surface used by REST.
type Gateway interface {
	Datasources() []string
	Schema(string) (schema.Snapshot, error)
	Execute(context.Context, query.Plan) (execution.Result, error)
	Explain(query.Plan) policy.Explanation
}

// GatewayBinder binds the authenticated principal before a REST operation.
type GatewayBinder func(principal.Trusted) (Gateway, error)

type errorEnvelope struct {
	Error errorDTO `json:"error"`
}
type errorDTO struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}
type queryResponse struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}
type explainResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}
type schemaResponse struct {
	Source    string        `json:"source"`
	Relations []relationDTO `json:"relations"`
}
type schemaListResponse struct {
	Sources []schemaResponse `json:"sources"`
}
type relationDTO struct {
	Schema  string      `json:"schema"`
	Name    string      `json:"name"`
	Columns []columnDTO `json:"columns"`
}
type columnDTO struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

type restAPI struct {
	bind     GatewayBinder
	recorder audit.Recorder
}

func newRESTAPI(bind GatewayBinder, recorder audit.Recorder) *restAPI {
	return &restAPI{bind: bind, recorder: recorder}
}

func (api *restAPI) routes(router chi.Router) {
	router.Post("/api/v1/query", api.query)
	router.Post("/api/v1/query/explain", api.explain)
	router.Get("/api/v1/schema", api.schemaList)
	router.Get("/api/v1/schema/{source}", api.schemaGet)
}

// execute keeps the operation audit before response commitment.
func (api *restAPI) execute(request *http.Request, operation audit.Operation, principal string, run func() error) error {
	return AuditedOperation(request, api.recorder, operation, principal, run)
}

func (api *restAPI) query(writer http.ResponseWriter, request *http.Request) {
	api.planOperation(writer, request, audit.OperationQuery, func(gateway Gateway, plan query.Plan) (any, error) {
		result, err := gateway.Execute(request.Context(), plan)
		return queryResponse{Columns: append([]string(nil), result.Columns...), Rows: result.Rows}, err
	})
}

func (api *restAPI) explain(writer http.ResponseWriter, request *http.Request) {
	api.planOperation(writer, request, audit.OperationExplain, func(gateway Gateway, plan query.Plan) (any, error) {
		explanation := gateway.Explain(plan)
		reason := "denied"
		if explanation.Allowed {
			reason = "allowed"
		}
		return explainResponse{Allowed: explanation.Allowed, Reason: reason}, nil
	})
}

func (api *restAPI) planOperation(writer http.ResponseWriter, request *http.Request, operation audit.Operation, call func(Gateway, query.Plan) (any, error)) {
	identity, ok := Principal(request.Context())
	if !ok || api.bind == nil {
		unauthorized(writer, request)
		return
	}
	var response any
	err := api.execute(request, operation, identity.Name(), func() error {
		plan, err := decodePlan(request, writer)
		if err != nil {
			return err
		}
		gateway, err := api.bind(identity)
		if err != nil || gateway == nil {
			return errors.New("runtime: gateway is unavailable")
		}
		response, err = call(gateway, plan.Plan)
		return err
	})
	annotateOutcome(request.Context(), err)
	if err != nil {
		writeMappedError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func (api *restAPI) schemaList(writer http.ResponseWriter, request *http.Request) {
	identity, ok := Principal(request.Context())
	if !ok || api.bind == nil {
		unauthorized(writer, request)
		return
	}
	response := schemaListResponse{Sources: make([]schemaResponse, 0)}
	err := api.execute(request, audit.OperationSchemaList, identity.Name(), func() error {
		gateway, err := api.bind(identity)
		if err != nil || gateway == nil {
			return errors.New("runtime: gateway is unavailable")
		}
		for _, source := range gateway.Datasources() {
			snapshot, err := gateway.Schema(source)
			if err != nil {
				return err
			}
			response.Sources = append(response.Sources, snapshotDTO(snapshot))
		}
		return nil
	})
	annotateOutcome(request.Context(), err)
	if err != nil {
		writeMappedError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func (api *restAPI) schemaGet(writer http.ResponseWriter, request *http.Request) {
	identity, ok := Principal(request.Context())
	if !ok || api.bind == nil {
		unauthorized(writer, request)
		return
	}
	var response schemaResponse
	err := api.execute(request, audit.OperationSchemaGet, identity.Name(), func() error {
		gateway, err := api.bind(identity)
		if err != nil || gateway == nil {
			return errors.New("runtime: gateway is unavailable")
		}
		source := chi.URLParam(request, "source")
		found := false
		for _, visible := range gateway.Datasources() {
			if visible == source {
				found = true
				break
			}
		}
		if !found {
			return errors.New("schema: metadata is unavailable")
		}
		snapshot, err := gateway.Schema(source)
		if err != nil {
			return errors.New("schema: metadata is unavailable")
		}
		response = snapshotDTO(snapshot)
		return nil
	})
	annotateOutcome(request.Context(), err)
	if err != nil {
		writeMappedError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func decodePlan(request *http.Request, writer http.ResponseWriter) (query.ValidatedPlan, error) {
	mediaType := request.Header.Get("Content-Type")
	if mediaType == "" || !strings.EqualFold(strings.TrimSpace(strings.Split(mediaType, ";")[0]), "application/json") {
		return query.ValidatedPlan{}, errors.New("query plan: invalid input")
	}
	request.Body = http.MaxBytesReader(writer, request.Body, MaxPlanBodyBytes)
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return query.ValidatedPlan{}, errors.New("query plan: invalid input")
	}
	return query.Decode(data)
}

func snapshotDTO(snapshot schema.Snapshot) schemaResponse {
	response := schemaResponse{Source: snapshot.Datasource(), Relations: make([]relationDTO, 0, len(snapshot.Relations()))}
	for _, relation := range snapshot.Relations() {
		dto := relationDTO{Schema: relation.Schema, Name: relation.Name, Columns: make([]columnDTO, 0, len(relation.Columns()))}
		for _, column := range relation.Columns() {
			dto.Columns = append(dto.Columns, columnDTO{Name: column.Name, Type: column.TypeName, Nullable: column.Nullable})
		}
		response.Relations = append(response.Relations, dto)
	}
	return response
}

func writeMappedError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, ErrAuditUnavailable) {
		writeError(writer, request, http.StatusServiceUnavailable, "unavailable", "service is unavailable")
		return
	}
	var validation *query.ValidationError
	if errors.As(err, &validation) || strings.HasPrefix(err.Error(), "query plan:") || strings.HasPrefix(err.Error(), "compiler:") {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "request is invalid")
		return
	}
	var executionError execution.Error
	if errors.As(err, &executionError) {
		switch executionError.Category {
		case execution.ErrorUnavailable:
			writeError(writer, request, http.StatusServiceUnavailable, "unavailable", "service is unavailable")
		case execution.ErrorTimeout:
			writeError(writer, request, http.StatusRequestTimeout, "timeout", "operation timed out")
		case execution.ErrorCapacity:
			writeError(writer, request, http.StatusTooManyRequests, "capacity", "capacity limit reached")
		case execution.ErrorRowLimit, execution.ErrorResultSize:
			writeError(writer, request, http.StatusUnprocessableEntity, "guardrail", "operation exceeds limits")
		default:
			writeError(writer, request, http.StatusInternalServerError, "internal", "internal server error")
		}
		return
	}
	switch {
	case strings.HasPrefix(err.Error(), "policy:"):
		writeError(writer, request, http.StatusForbidden, "denied", "operation is not allowed")
	case strings.HasPrefix(err.Error(), "schema:"):
		writeError(writer, request, http.StatusNotFound, "not_found", "resource was not found")
	case strings.HasPrefix(err.Error(), "runtime:"):
		writeError(writer, request, http.StatusServiceUnavailable, "unavailable", "service is unavailable")
	default:
		writeError(writer, request, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func annotateOutcome(ctx context.Context, err error) {
	if err == nil {
		telemetry.SetPolicy(ctx, "allowed")
		telemetry.SetExecution(ctx, "success")
		return
	}
	if strings.HasPrefix(err.Error(), "policy:") {
		telemetry.SetPolicy(ctx, "denied")
		telemetry.SetExecution(ctx, "not_applicable")
		return
	}
	var executionError execution.Error
	if errors.As(err, &executionError) {
		telemetry.SetPolicy(ctx, "allowed")
		switch executionError.Category {
		case execution.ErrorTimeout:
			telemetry.SetExecution(ctx, "timeout")
		case execution.ErrorCapacity:
			telemetry.SetExecution(ctx, "capacity")
		case execution.ErrorRowLimit:
			telemetry.SetExecution(ctx, "row_limit")
		case execution.ErrorResultSize:
			telemetry.SetExecution(ctx, "result_size")
		case execution.ErrorUnavailable:
			telemetry.SetExecution(ctx, "unavailable")
		default:
			telemetry.SetExecution(ctx, "internal")
		}
		return
	}
	if strings.HasPrefix(err.Error(), "runtime:") || errors.Is(err, ErrAuditUnavailable) {
		telemetry.SetExecution(ctx, "unavailable")
		return
	}
	telemetry.SetExecution(ctx, "internal")
}

func writeError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	writeJSON(writer, status, errorEnvelope{Error: errorDTO{Code: code, Message: message, RequestID: RequestID(request.Context())}})
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
