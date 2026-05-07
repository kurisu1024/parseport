package mime

import (
	"fmt"
	"io"
	"unicode/utf8"
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
		return &utf16Reader{r: r, bigEndian: true}, nil
	case LATIN1:
		return &latin1Reader{r: r, inBuf: make([]byte, 512)}, nil
	default:
		return nil, fmt.Errorf("unsupported charset: %q", charset)
	}
}

type latin1Reader struct {
	r      io.Reader
	inBuf  []byte
	inN    int
	inOff  int
	outBuf [2]byte
	outN   int
	outOff int
}

func (lr *latin1Reader) Read(p []byte) (int, error) {
	written := 0

	if lr.outN > lr.outOff {
		n := copy(p[written:], lr.outBuf[lr.outOff:lr.outN])
		written += n
		lr.outOff += n
		if lr.outOff == lr.outN {
			lr.outN = 0
			lr.outOff = 0
		}
		if written == len(p) {
			return written, nil
		}
	}

	for written < len(p) {
		if lr.inOff >= lr.inN {
			n, err := lr.r.Read(lr.inBuf)
			lr.inN = n
			lr.inOff = 0
			if n == 0 {
				if written > 0 {
					return written, nil
				}
				return 0, err
			}
		}

		b := lr.inBuf[lr.inOff]
		lr.inOff++

		if b < 0x80 {
			p[written] = b
			written++
		} else {
			b1 := byte(0xC0 | (b >> 6))
			b2 := byte(0x80 | (b & 0x3F))
			p[written] = b1
			written++
			if written < len(p) {
				p[written] = b2
				written++
			} else {
				lr.outBuf[0] = b2
				lr.outN = 1
				lr.outOff = 0
				break
			}
		}
	}

	return written, nil
}

type utf16Reader struct {
	r         io.Reader
	bigEndian bool
	bomRead   bool
	outBuf    [4]byte
	outN      int
	outOff    int
}

func (ur *utf16Reader) Read(p []byte) (int, error) {
	written := 0

	if ur.outN > ur.outOff {
		n := copy(p[written:], ur.outBuf[ur.outOff:ur.outN])
		written += n
		ur.outOff += n
		if ur.outOff == ur.outN {
			ur.outN = 0
			ur.outOff = 0
		}
		if written == len(p) {
			return written, nil
		}
	}

	var b [2]byte
	for written < len(p) {
		_, err := io.ReadFull(ur.r, b[:])
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			if written > 0 {
				return written, nil
			}
			return 0, io.EOF
		}
		if err != nil {
			return written, err
		}

		if !ur.bomRead {
			ur.bomRead = true
			if b[0] == 0xFE && b[1] == 0xFF {
				ur.bigEndian = true
				continue
			}
			if b[0] == 0xFF && b[1] == 0xFE {
				ur.bigEndian = false
				continue
			}
		}

		var cu uint16
		if ur.bigEndian {
			cu = uint16(b[0])<<8 | uint16(b[1])
		} else {
			cu = uint16(b[1])<<8 | uint16(b[0])
		}

		var r rune
		if cu >= 0xD800 && cu <= 0xDBFF {
			var lo [2]byte
			_, err := io.ReadFull(ur.r, lo[:])
			if err != nil {
				return written, fmt.Errorf("invalid utf-16 surrogate: missing low surrogate: %w", err)
			}
			var locu uint16
			if ur.bigEndian {
				locu = uint16(lo[0])<<8 | uint16(lo[1])
			} else {
				locu = uint16(lo[1])<<8 | uint16(lo[0])
			}
			if locu < 0xDC00 || locu > 0xDFFF {
				return written, fmt.Errorf("invalid utf-16 surrogate: expected low surrogate, got 0x%04X", locu)
			}
			r = rune(0x10000) + rune(cu-0xD800)*0x400 + rune(locu-0xDC00)
		} else if cu >= 0xDC00 && cu <= 0xDFFF {
			return written, fmt.Errorf("invalid utf-16 surrogate: unexpected low surrogate 0x%04X", cu)
		} else {
			r = rune(cu)
		}

		n := utf8.EncodeRune(ur.outBuf[:], r)
		ur.outN = n
		ur.outOff = 0

		nc := copy(p[written:], ur.outBuf[:ur.outN])
		written += nc
		ur.outOff += nc
		if ur.outOff == ur.outN {
			ur.outN = 0
			ur.outOff = 0
		}
	}

	return written, nil
}
