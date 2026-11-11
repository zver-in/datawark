package query

import "testing"

func FuzzDecode(f *testing.F) {
	f.Add([]byte(`{"source":"analytics","table":"reporting.orders","projection":["id"],"limit":1}`))
	f.Add([]byte(`{"filters":[{"value":{}}]}`))
	f.Add([]byte("not json"))
	f.Fuzz(func(t *testing.T, input []byte) {
		validated, err := Decode(input)
		if err != nil {
			return
		}
		if _, err := Validate(validated.Plan); err != nil {
			t.Fatalf("Decode returned an unsupported plan: %v", err)
		}
	})
}
