package address

import (
	"io"

	"strings"
)

// Address represents a parsed RFC 5321/5322 email address.
type Address struct {
	// DisplayName holds the RFC 5322 display name (the phrase before angle brackets).
	// Empty for bare addr-spec addresses with no name-addr wrapper.
	DisplayName string

	// Localpart is the part of the address before "@".
	// For quoted local-parts, backslash escapes are resolved.
	Localpart string

	// Domain is the part of the address after "@".
	// For domain-literals, this includes the surrounding brackets, e.g. "[192.0.2.1]".
	Domain string
}

// ParseAddress is a helper that wraps addr in an io.Reader and calls ReadAddress.
func ParseAddress(addr string) (Address, error) {
	return ReadAddress(strings.NewReader(addr))
}

// ReadAddress parses a single RFC 5321/5322 email address from r.
func ReadAddress(r io.Reader) (Address, error) {
	return newParser(r).parse()
}

func ReadAddressConcurrentLexer(r io.Reader) (Address, error) {
	return newConcurrentParser(r)
}
