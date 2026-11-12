package mcpserver

import (
	"context"
	"datawark/internal/execution"
	"datawark/internal/policy"
	"datawark/internal/query"
	"datawark/internal/schema"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type gatewayFake struct {
	calls int
}

func (fake *gatewayFake) Datasources() []string { return []string{"visible"} }
func (fake *gatewayFake) Schema(name string) (schema.Snapshot, error) {
	if name != "visible" {
		return schema.Snapshot{}, context.Canceled
	}
	return schema.NewSnapshot("visible", []schema.Relation{schema.NewRelation("public", "orders", []schema.Column{{Name: "id", TypeName: "integer"}})}), nil
}
func (fake *gatewayFake) Execute(_ context.Context, plan query.Plan) (execution.Result, error) {
	fake.calls++
	if plan.Source != "visible" || plan.Table != "public.orders" {
		return execution.Result{}, context.Canceled
	}
	return execution.Result{Columns: []string{"id"}, Rows: [][]any{{int64(7)}}}, nil
}
func (fake *gatewayFake) Explain(plan query.Plan) policy.Explanation {
	if plan.Table == "public.orders" {
		return policy.Explanation{Allowed: true, Reason: policy.ReasonAllowed, PolicyID: "visible-read"}
	}
	return policy.Explanation{Reason: policy.ReasonDenied}
}

func TestProtocolToolsAreStrictAndPolicyBound(t *testing.T) {
	ctx := context.Background()
	fake := &gatewayFake{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := New(fake)
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		got = append(got, tool.Name)
	}
	slices.Sort(got)
	if strings.Join(got, ",") != "explain_query,list_allowed_schema,query_data" {
		t.Fatalf("tools = %v", got)
	}

	schemaResult := call(t, ctx, session, "list_allowed_schema", map[string]any{})
	if schemaResult.IsError || !strings.Contains(text(t, schemaResult), "visible") || strings.Contains(text(t, schemaResult), "hidden") {
		t.Fatalf("schema result = %#v", schemaResult)
	}

	allowed := map[string]any{"source": "visible", "table": "public.orders", "projection": []string{"id"}, "limit": 1}
	queryResult := call(t, ctx, session, "query_data", allowed)
	if queryResult.IsError || !strings.Contains(text(t, queryResult), "columns") || fake.calls != 1 {
		t.Fatalf("query result = %#v calls=%d", queryResult, fake.calls)
	}

	for _, invalid := range []map[string]any{
		{"source": "visible", "table": "public.orders", "projection": []string{"id"}, "limit": 1, "rawSql": "select *"},
		{"source": "visible", "table": "public.orders", "projection": []string{"id"}, "limit": 1, "principal": "admin"},
		{"source": "visible", "table": "public.orders", "projection": []string{"id"}, "limit": 1, "columns": []string{"id"}},
	} {
		if result := call(t, ctx, session, "query_data", invalid); !result.IsError || fake.calls != 1 {
			t.Fatalf("invalid result=%#v calls=%d", result, fake.calls)
		}
	}
	denied := call(t, ctx, session, "explain_query", map[string]any{"source": "visible", "table": "public.hidden", "projection": []string{"id"}, "limit": 1})
	if denied.IsError || strings.Contains(text(t, denied), "PolicyID") || !strings.Contains(text(t, denied), "denied") || fake.calls != 1 {
		t.Fatalf("denied result=%#v calls=%d", denied, fake.calls)
	}
}

func call(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func text(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v", result.Content)
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %#v", result.Content[0])
	}
	var value any
	if !result.IsError && json.Unmarshal([]byte(content.Text), &value) != nil {
		t.Fatalf("invalid JSON content: %q", content.Text)
	}
	return content.Text
}
