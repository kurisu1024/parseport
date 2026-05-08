package mime

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
)

// aliases maps non-IANA charset names to their IANA-registered equivalents.
var aliases = map[string]string{
	"latin-1":  "iso-8859-1",
	"latin-2":  "iso-8859-2",
	"latin-3":  "iso-8859-3",
	"latin-4":  "iso-8859-4",
	"latin-5":  "iso-8859-9",
	"latin-6":  "iso-8859-10",
	"latin-9":  "iso-8859-15",
	"latin-10": "iso-8859-16",
}

func NewCharsetDecoder(charset string, r io.Reader) (io.Reader, error) {
	name := strings.ToLower(strings.TrimSpace(charset))
	if alias, ok := aliases[name]; ok {
		name = alias
	}
	if name == "utf-8" || name == "us-ascii" || name == "ascii" {
		return r, nil
	}
	enc, err := ianaindex.MIME.Encoding(name)
	if err != nil {
		return nil, fmt.Errorf("unsupported charset: %q", charset)
	}
	return transform.NewReader(r, enc.NewDecoder()), nil
}
