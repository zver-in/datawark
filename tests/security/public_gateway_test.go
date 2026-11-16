package security

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func endpoint(t *testing.T) string {
	t.Helper()
	value := os.Getenv("DATAWARK_PUBLIC_ENDPOINT")
	if value == "" {
		t.Skip("requires make verify-public-mvp")
	}
	return value
}

func apiKey(t *testing.T) string {
	t.Helper()
	value := os.Getenv("DATAWARK_PUBLIC_API_KEY")
	if value == "" {
		t.Skip("requires make verify-public-mvp")
	}
	return value
}

func request(t *testing.T, method, path, body string, authorization ...string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, endpoint(t)+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, value := range authorization {
		req.Header.Add("Authorization", value)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func read(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func assertNoProtectedValue(t *testing.T, body string) {
	t.Helper()
	for _, value := range []string{os.Getenv("DATAWARK_PUBLIC_PROTECTED_VALUE"), apiKey(t)} {
		if value != "" && strings.Contains(body, value) {
			t.Fatalf("response leaked protected value %q: %s", value, body)
		}
	}
}

func TestRESTSecurityBoundaries(t *testing.T) {
	ready := request(t, http.MethodGet, "/api/v1/ready", "")
	if body := read(t, ready); ready.StatusCode != http.StatusOK {
		t.Fatalf("readiness status=%d body=%s", ready.StatusCode, body)
	}

	allowed := `{"source":"analytics","table":"sales.orders","projection":["id","status","total","created_at"],"filters":[],"order":[],"limit":1}`
	hiddenRelation := `{"source":"analytics","table":"sales.hidden_orders","projection":["id"],"filters":[],"order":[],"limit":1}`
	hiddenColumn := `{"source":"analytics","table":"sales.orders","projection":["internal_note"],"filters":[],"order":[],"limit":1}`
	clientIdentity := `{"source":"analytics","table":"sales.orders","projection":["id"],"filters":[],"order":[],"limit":1,"principal":"admin"}`

	response := request(t, http.MethodPost, "/api/v1/query", allowed, "Bearer "+apiKey(t))
	if body := read(t, response); response.StatusCode != http.StatusOK || !strings.Contains(body, `"id"`) {
		t.Fatalf("allowed status=%d body=%s", response.StatusCode, body)
	}
	for _, test := range []struct {
		name    string
		body    string
		headers []string
		status  int
	}{
		{"missing", allowed, nil, http.StatusUnauthorized},
		{"malformed", allowed, []string{"Basic invalid"}, http.StatusUnauthorized},
		{"multiple", allowed, []string{"Bearer " + apiKey(t), "Bearer another"}, http.StatusUnauthorized},
		{"unknown", allowed, []string{"Bearer unknown-credential"}, http.StatusUnauthorized},
		{"client identity", clientIdentity, []string{"Bearer " + apiKey(t)}, http.StatusBadRequest},
		{"hidden relation", hiddenRelation, []string{"Bearer " + apiKey(t)}, http.StatusForbidden},
		{"hidden column", hiddenColumn, []string{"Bearer " + apiKey(t)}, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, http.MethodPost, "/api/v1/query", test.body, test.headers...)
			body := read(t, response)
			if response.StatusCode != test.status || !strings.Contains(body, `"error"`) {
				t.Fatalf("status=%d body=%s", response.StatusCode, body)
			}
			assertNoProtectedValue(t, body)
		})
	}
}

type bearerTransport struct{ key string }

func (transport bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+transport.key)
	return http.DefaultTransport.RoundTrip(request)
}

func TestMCPAndObservabilityBoundaries(t *testing.T) {
	client := mcp.NewClient(&mcp.Implementation{Name: "public-gateway-release-test", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: endpoint(t) + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{key: apiKey(t)}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "query_data", Arguments: map[string]any{"source": "analytics", "table": "sales.orders", "projection": []string{"id"}, "limit": 1}})
	if err != nil || result.IsError {
		t.Fatalf("MCP result=%#v err=%v", result, err)
	}

	denied := request(t, http.MethodPost, "/api/v1/query", `{"source":"analytics","table":"sales.orders","projection":["internal_note"],"filters":[],"order":[],"limit":1}`, "Bearer "+apiKey(t))
	deniedBody := read(t, denied)
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("denied status=%d body=%s", denied.StatusCode, deniedBody)
	}
	metrics := request(t, http.MethodGet, "/metrics", "")
	metricsBody := read(t, metrics)
	if metrics.StatusCode != http.StatusOK {
		t.Fatalf("metrics status=%d", metrics.StatusCode)
	}
	for _, value := range []string{apiKey(t), os.Getenv("DATAWARK_PUBLIC_PROTECTED_VALUE")} {
		if strings.Contains(metricsBody, value) {
			t.Fatalf("metrics leaked %q", value)
		}
	}
	project := os.Getenv("DATAWARK_PUBLIC_COMPOSE_PROJECT")
	if project == "" {
		t.Fatal("DATAWARK_PUBLIC_COMPOSE_PROJECT must be set by make verify-public-mvp")
	}
	logs, err := exec.Command("docker", "compose", "--project-name", project, "logs", "--no-color", "gateway").CombinedOutput()
	if err != nil || !strings.Contains(string(logs), `"version":"v1alpha1"`) || !strings.Contains(string(logs), "http request complete") {
		t.Fatalf("Compose logs missing audit or operational evidence: %s err=%v", logs, err)
	}
	for _, value := range []string{apiKey(t), os.Getenv("DATAWARK_PUBLIC_PROTECTED_VALUE")} {
		if strings.Contains(string(logs), value) {
			t.Fatalf("Compose logs leaked %q", value)
		}
	}
}

func TestDatabaseRoleCannotWriteAndDataRemainsStable(t *testing.T) {
	project := os.Getenv("DATAWARK_PUBLIC_COMPOSE_PROJECT")
	reader := os.Getenv("DATAWARK_PUBLIC_DB_READER")
	password := os.Getenv("DATAWARK_PUBLIC_DB_PASSWORD")
	if project == "" || reader == "" || password == "" {
		t.Skip("requires make verify-public-mvp")
	}
	command := exec.Command("docker", "compose", "--project-name", project, "exec", "-T", "-e", "PGPASSWORD="+password, "postgres", "psql", "-h", "127.0.0.1", "-U", reader, "-d", "datawark_demo", "-c", "UPDATE sales.orders SET status = 'tampered' WHERE id = 1001")
	if output, err := command.CombinedOutput(); err == nil || bytes.Contains(output, []byte("UPDATE 1")) {
		t.Fatalf("reader write unexpectedly succeeded: %s", output)
	}
	query := exec.Command("docker", "compose", "--project-name", project, "exec", "-T", "postgres", "psql", "-U", "datawark_admin", "-d", "datawark_demo", "-tAc", "SELECT status FROM sales.orders WHERE id = 1001")
	output, err := query.Output()
	if err != nil || strings.TrimSpace(string(output)) != "paid" {
		t.Fatalf("seeded data changed: %q err=%v", output, err)
	}
	var response map[string]any
	body := `{"source":"analytics","table":"sales.orders","projection":["internal_note"],"filters":[],"order":[],"limit":1}`
	denied := request(t, http.MethodPost, "/api/v1/query", body, "Bearer "+apiKey(t))
	data := read(t, denied)
	if denied.StatusCode != http.StatusForbidden || json.Unmarshal([]byte(data), &response) != nil {
		t.Fatalf("negative Gateway request status=%d body=%s", denied.StatusCode, data)
	}
}
