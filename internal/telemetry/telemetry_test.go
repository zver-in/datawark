package telemetry

import (
	"context"
	"testing"
)

func TestOutcomeUsesBoundedPrecedence(t *testing.T) {
	for _, test := range []struct {
		name, auth, policy, execution, want string
		status                              int
	}{
		{"success", "authenticated", "allowed", "success", "success", 200},
		{"unauthenticated wins", "unauthenticated", "denied", "internal", "unauthenticated", 500},
		{"denied wins", "authenticated", "denied", "timeout", "denied", 408},
		{"timeout", "authenticated", "allowed", "timeout", "timeout", 500},
		{"capacity", "authenticated", "allowed", "capacity", "capacity", 500},
		{"row limit", "authenticated", "allowed", "row_limit", "row_limit", 422},
		{"result size", "authenticated", "allowed", "result_size", "result_size", 422},
		{"unavailable", "authenticated", "allowed", "unavailable", "unavailable", 500},
		{"internal", "authenticated", "allowed", "internal", "internal", 200},
		{"unknown annotations", "hostile", "hostile", "hostile", "other", 500},
		{"unknown status", "authenticated", "not_applicable", "not_applicable", "other", 418},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := WithRequest(context.Background(), TransportREST, "query")
			SetAuthentication(ctx, test.auth)
			SetPolicy(ctx, test.policy)
			SetExecution(ctx, test.execution)
			if got := Outcome(ctx, test.status); got != test.want {
				t.Fatalf("Outcome() = %q, want %q", got, test.want)
			}
		})
	}
}
