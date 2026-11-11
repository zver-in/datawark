package query

import (
	"strings"
	"testing"
)

func TestDecodeAcceptsBoundedPlan(t *testing.T) {
	t.Parallel()
	input := []byte(`{"source":"analytics","table":"reporting.orders","projection":["id"],"filters":[{"column":"amount","operator":"gte","value":3.5}],"order":[{"column":"id","direction":"asc"}],"limit":10}`)
	validated, err := Decode(input)
	if err != nil {
		t.Fatal(err)
	}
	if validated.Plan.Filters[0].Value.Kind != ScalarNumber || validated.Plan.Filters[0].Value.Number != 3.5 {
		t.Fatalf("scalar was not decoded: %#v", validated.Plan.Filters[0].Value)
	}
}

func TestDecodeRejectsUnsupportedContentWithoutReflection(t *testing.T) {
	t.Parallel()
	secret := "hostile_payload_3241"
	tests := []string{
		`{"source":"analytics","table":"reporting.orders","projection":["id"],"limit":1,"rawSql":"` + secret + `"}`,
		`{"source":"analytics","table":"reporting.orders","projection":["id"],"limit":1,"joins":["` + secret + `"]}`,
		`{"source":"analytics","table":"reporting.orders","projection":["id"],"limit":1,"groupBy":["` + secret + `"]}`,
		`{"source":"analytics","table":"reporting.orders","projection":["id"],"limit":1,"filters":[{"column":"id","operator":"eq","value":{"function":"` + secret + `"}}]}`,
		`{"source":"analytics","table":"reporting.orders,other.items","projection":["id"],"limit":1}`,
		`{"source":"analytics","table":"reporting.orders","projection":["id"],"limit":1.5}`,
	}
	for _, input := range tests {
		_, err := Decode([]byte(input))
		if err == nil {
			t.Fatalf("Decode(%s) succeeded", input)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("Decode() reflected hostile input: %v", err)
		}
	}
}
