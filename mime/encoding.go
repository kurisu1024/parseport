package mime

import (
	"fmt"
	"io"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	UTF8   = "utf-8"
	UTF16  = "utf-16"
	LATIN1 = "latin-1"
)

func NewCharsetDecoder(charset string, r io.Reader) (io.Reader, error) {
	switch charset {
	case UTF8:
		return r, nil
	case UTF16:
		dec := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder()
		return transform.NewReader(r, dec), nil
	case LATIN1:
		return transform.NewReader(r, charmap.ISO8859_1.NewDecoder()), nil
	default:
		return nil, fmt.Errorf("unsupported charset: %q", charset)
	}
}
