package address

import "fmt"

// ParseError is returned by ParseAddress and ReadAddress when the input is
// malformed. Pos is the 0-based byte offset where the error was detected.
// Callers can recover position info with errors.As(err, &perr).
type ParseError struct {
	Pos     int    // 0-based byte offset in the input stream
	Got     byte   // offending byte; 0 when the error is an unexpected EOF
	Context string // grammar production being parsed (e.g. "local-part", "domain")
	Msg     string // human-readable description
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("address: parse error at offset %d: %s (while parsing %s)", e.Pos, e.Msg, e.Context)
}
