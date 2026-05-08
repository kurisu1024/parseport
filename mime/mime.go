package mime

import (
	"io"

	"github.com/kurisu2024/parseport/header"
)

// TODO: Implement MIME parsing
func ParseMIME(r io.Reader) (*MIME, error) {
	return nil, nil
}

type MIME struct {
	Header []header.Header
	Parts  []Part
}

type Part struct {
	Headers []header.Header
	body    io.Reader
}
