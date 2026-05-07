package header

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/textproto"
)

type Header struct {
	name  string
	value string
	raw   []byte
}

// ParseHeaders parses a series of headers from the Reader.
func ParseHeaders(r io.Reader) ([]Header, error) {
	br := bufio.NewReader(r)
	var headers []Header

	for {
		line, err := br.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return headers, err
		}

		// Empty line signals end of headers.
		trimmed := bytes.TrimRight(line, "\r\n")
		if len(trimmed) == 0 {
			break
		}

		raw := make([]byte, len(line))
		copy(raw, line)

		// Collect continuation lines (folded header).
		for {
			b, peekErr := br.Peek(1)
			if peekErr != nil || (b[0] != ' ' && b[0] != '\t') {
				break
			}
			cont, contErr := br.ReadBytes('\n')
			raw = append(raw, cont...)
			if contErr != nil {
				break
			}
		}

		colonIdx := bytes.IndexByte(trimmed, ':')
		if colonIdx < 0 {
			return headers, fmt.Errorf("header: malformed header line %q: missing ':'", trimmed)
		}

		name := textproto.CanonicalMIMEHeaderKey(string(bytes.TrimSpace(trimmed[:colonIdx])))
		value := unfold(raw[colonIdx+1:])

		headers = append(headers, Header{
			name:  name,
			value: value,
			raw:   raw,
		})

		if err == io.EOF {
			break
		}
	}

	return headers, nil
}

// unfold removes RFC 5322 folding from a header value: strips leading WSP,
// removes CRLF (or bare LF) immediately followed by WSP, and trims trailing line endings.
func unfold(b []byte) string {
	b = bytes.TrimLeft(b, " \t")

	var out []byte
	for i := 0; i < len(b); i++ {
		if b[i] == '\r' && i+1 < len(b) && b[i+1] == '\n' {
			if i+2 < len(b) && (b[i+2] == ' ' || b[i+2] == '\t') {
				// Folded line: skip CRLF, keep the WSP on the next line.
				i++
				continue
			}
		} else if b[i] == '\n' {
			if i+1 < len(b) && (b[i+1] == ' ' || b[i+1] == '\t') {
				// LF-only folding: skip LF, keep the WSP.
				continue
			}
		}
		out = append(out, b[i])
	}

	return string(bytes.TrimRight(out, "\r\n"))
}

// decode decodes an encoded header value (e.g. RFC 2047 encoded-words).
// TODO: implement RFC 2047 decoding.
func decode(value string) (string, error) {
	return value, nil
}

// DecodeHeader decodes a list of headers and normalizes the header names for
// easy referencing. Note, it is possible for there to be more than one of the same
// header.
func DecodeHeader(hdrs ...Header) (map[string][]string, error) {
	decoded := make(map[string][]string)
	for _, hdr := range hdrs {
		if decoded[hdr.name] == nil {
			decoded[hdr.name] = make([]string, 0, 1)
		}
		v, err := decode(hdr.value)
		if err != nil {
			return decoded, err
		}
		decoded[hdr.name] = append(decoded[hdr.name], v)
	}
	return decoded, nil
}
