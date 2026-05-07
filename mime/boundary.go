package mime

import (
	"bytes"
	"io"
)

type boundaryReader struct {
	needle []byte
	r      io.Reader
	buf    []byte
	start  int
	end    int
	eof    bool
	closed bool
}

func NewBoundaryReader(r io.Reader, boundary string) io.Reader {
	needle := []byte("\r\n--" + boundary)
	bufSize := 2 * len(needle)
	if bufSize < 512 {
		bufSize = 512
	}
	return &boundaryReader{
		needle: needle,
		r:      r,
		buf:    make([]byte, bufSize),
	}
}

func (br *boundaryReader) fill() error {
	copy(br.buf, br.buf[br.start:br.end])
	br.end -= br.start
	br.start = 0

	if br.end == len(br.buf) {
		newBuf := make([]byte, len(br.buf)*2)
		copy(newBuf, br.buf[:br.end])
		br.buf = newBuf
	}

	n, err := br.r.Read(br.buf[br.end:])
	br.end += n
	if err == io.EOF {
		br.eof = true
		return nil
	}
	return err
}

func (br *boundaryReader) Read(p []byte) (int, error) {
	if br.closed {
		return 0, io.EOF
	}

	for br.end-br.start < len(br.needle) && !br.eof {
		if err := br.fill(); err != nil {
			return 0, err
		}
	}

search:
	data := br.buf[br.start:br.end]

	idx := bytes.Index(data, br.needle)
	switch {
	case idx > 0:
		n := copy(p, data[:idx])
		br.start += n
		return n, nil

	case idx == 0:
		br.closed = true
		return 0, io.EOF

	case br.eof:
		n := copy(p, data)
		br.start += n
		if br.start == br.end {
			return n, io.EOF
		}
		return n, nil

	default:
		safe := len(data) - (len(br.needle) - 1)
		if safe <= 0 {
			if err := br.fill(); err != nil {
				return 0, err
			}
			goto search
		}
		n := copy(p, data[:safe])
		br.start += n
		return n, nil
	}
}
