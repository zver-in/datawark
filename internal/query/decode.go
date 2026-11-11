package query

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
)

// Decode strictly decodes a JSON query plan and validates its bounded grammar.
// Decode errors deliberately do not include untrusted input.
func Decode(data []byte) (ValidatedPlan, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var plan Plan
	if err := decoder.Decode(&plan); err != nil {
		return ValidatedPlan{}, errors.New("query plan: invalid input")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ValidatedPlan{}, errors.New("query plan: expected exactly one JSON value")
	}
	return Validate(plan)
}

// UnmarshalJSON permits only JSON scalar values for a filter value.
func (value *Scalar) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var raw any
	if err := decoder.Decode(&raw); err != nil {
		return errors.New("invalid scalar")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("invalid scalar")
	}
	switch typed := raw.(type) {
	case nil:
		*value = Null()
	case string:
		*value = String(typed)
	case bool:
		*value = Boolean(typed)
	case json.Number:
		number, err := typed.Float64()
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return errors.New("invalid scalar")
		}
		*value = Number(number)
	default:
		return errors.New("invalid scalar")
	}
	return nil
}
