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

func TestDecodeAcceptsCanonicalPublicFields(t *testing.T) {
	t.Parallel()

	_, err := Decode([]byte(`{"source":"analytics","table":"reporting.orders","projection":["id"],"order":[{"column":"id","direction":"asc"}],"limit":1}`))
	if err != nil {
		t.Fatalf("Decode() returned an error for canonical public fields: %v", err)
	}
}

func TestDecodeRejectsLegacyPublicFieldAliases(t *testing.T) {
	t.Parallel()

	for _, alias := range []string{"select", "columns", "orderBy", "order_by"} {
		alias := alias
		t.Run(alias, func(t *testing.T) {
			t.Parallel()
			input := []byte(`{"source":"analytics","table":"reporting.orders","projection":["id"],"limit":1,"` + alias + `":[]}`)
			if _, err := Decode(input); err == nil {
				t.Fatalf("Decode() accepted legacy alias %q", alias)
			}
		})
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
