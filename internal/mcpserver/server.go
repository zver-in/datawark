// Package mcpserver adapts the Gateway runtime to the local MCP stdio protocol.
package mcpserver

import (
	"bytes"
	"context"
	"datawark/internal/execution"
	"datawark/internal/policy"
	"datawark/internal/query"
	"datawark/internal/runtime"
	"datawark/internal/schema"
	"encoding/json"
	"errors"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Gateway is the deliberately small runtime surface exposed to MCP tools.
type Gateway interface {
	Datasources() []string
	Schema(string) (schema.Snapshot, error)
	Execute(context.Context, query.Plan) (execution.Result, error)
	Explain(query.Plan) policy.Explanation
}

// New constructs an MCP server with precisely the supported data tools.
func New(gateway Gateway) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "datawark", Version: "v1"}, nil)
	server.AddTool(&mcp.Tool{Name: "list_allowed_schema", Description: "List policy-visible datasources, relations, and columns.", InputSchema: emptyObjectSchema()}, schemaTool(gateway))
	server.AddTool(&mcp.Tool{Name: "query_data", Description: "Run one policy-enforced bounded read query.", InputSchema: planSchema()}, queryTool(gateway))
	server.AddTool(&mcp.Tool{Name: "explain_query", Description: "Explain whether one bounded read query is allowed.", InputSchema: planSchema()}, explainTool(gateway))
	return server
}

// Run serves the constructed server over the process stdin and stdout only.
func Run(ctx context.Context, gateway *runtime.Runtime) error {
	if gateway == nil {
		return errors.New("MCP runtime is unavailable")
	}
	return New(gateway).Run(ctx, &mcp.StdioTransport{})
}

func schemaTool(gateway Gateway) mcp.ToolHandler {
	return func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := emptyArguments(request.Params.Arguments); err != nil {
			return toolError("invalid input"), nil
		}
		result := schemaResult{Datasources: make([]datasourceResult, 0)}
		for _, datasource := range gateway.Datasources() {
			snapshot, err := gateway.Schema(datasource)
			if err != nil {
				continue
			}
			entry := datasourceResult{Name: snapshot.Datasource(), Relations: make([]relationResult, 0)}
			for _, relation := range snapshot.Relations() {
				columns := make([]columnResult, 0, len(relation.Columns()))
				for _, column := range relation.Columns() {
					columns = append(columns, columnResult{Name: column.Name, Type: column.TypeName, Nullable: column.Nullable})
				}
				entry.Relations = append(entry.Relations, relationResult{Name: relation.QualifiedName(), Columns: columns})
			}
			result.Datasources = append(result.Datasources, entry)
		}
		return toolResult(result), nil
	}
}

func queryTool(gateway Gateway) mcp.ToolHandler {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		plan, err := decodePlan(request.Params.Arguments)
		if err != nil {
			return toolError("invalid query plan"), nil
		}
		result, err := gateway.Execute(ctx, plan)
		if err != nil {
			return toolError("query unavailable"), nil
		}
		return toolResult(queryResult{Columns: result.Columns, Rows: result.Rows}), nil
	}
}

func explainTool(gateway Gateway) mcp.ToolHandler {
	return func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		plan, err := decodePlan(request.Params.Arguments)
		if err != nil {
			return toolError("invalid query plan"), nil
		}
		explanation := gateway.Explain(plan)
		result := explainResult{Allowed: explanation.Allowed, Reason: string(explanation.Reason)}
		if explanation.Allowed {
			result.PolicyID = explanation.PolicyID
		}
		return toolResult(result), nil
	}
}

func decodePlan(arguments json.RawMessage) (query.Plan, error) {
	validated, err := query.Decode(arguments)
	if err != nil {
		return query.Plan{}, errors.New("invalid query plan")
	}
	return validated.Plan, nil
}

func emptyArguments(arguments json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.DisallowUnknownFields()
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil || len(object) != 0 {
		return errors.New("invalid input")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("invalid input")
	}
	return nil
}

func toolResult(value any) *mcp.CallToolResult {
	encoded, err := json.Marshal(value)
	if err != nil {
		return toolError("internal error")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, StructuredContent: value}
}

func toolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}, IsError: true}
}

func emptyObjectSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false}
}

func planSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"source", "table", "projection", "limit"},
		"properties": map[string]any{
			"source": map[string]any{"type": "string"}, "table": map[string]any{"type": "string"}, "projection": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"filters": map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"column", "operator", "value"}, "properties": map[string]any{"column": map[string]any{"type": "string"}, "operator": map[string]any{"enum": []string{"eq", "neq", "lt", "lte", "gt", "gte"}}, "value": map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "number"}, map[string]any{"type": "boolean"}, map[string]any{"type": "null"}}}}}},
			"order":   map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"column", "direction"}, "properties": map[string]any{"column": map[string]any{"type": "string"}, "direction": map[string]any{"enum": []string{"asc", "desc"}}}}}, "limit": map[string]any{"type": "integer"},
		},
	}
}

type schemaResult struct {
	Datasources []datasourceResult `json:"datasources"`
}
type datasourceResult struct {
	Name      string           `json:"name"`
	Relations []relationResult `json:"relations"`
}
type relationResult struct {
	Name    string         `json:"name"`
	Columns []columnResult `json:"columns"`
}
type columnResult struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}
type queryResult struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}
type explainResult struct {
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason"`
	PolicyID string `json:"policyId,omitempty"`
}
