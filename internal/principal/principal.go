// Package principal defines the server-side identity value used to authorize
// runtime operations. It is deliberately separate from untrusted query input.
package principal

import (
	"errors"
	"strings"
)

// Trusted is an opaque, validated principal established at a server-side
// identity boundary.
type Trusted struct{ name string }

// New creates a trusted principal from an identity already verified by a
// trusted adapter.
func New(name string) (Trusted, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Trusted{}, errors.New("trusted principal is required")
	}
	return Trusted{name: name}, nil
}

// Name returns the policy principal represented by this trusted value.
func (principal Trusted) Name() string { return principal.name }

// Valid reports whether the value was created by New.
func (principal Trusted) Valid() bool { return principal.name != "" }
