package mime_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	mime "github.com/kurisu2024/parseport/mime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoundaryReader(t *testing.T) {
	tests := []struct {
		name     string
		input    io.Reader
		boundary string
		want     []byte
	}{
		{
			name:     "single part reads bytes before boundary",
			input:    strings.NewReader("hello world\r\n--bound--"),
			boundary: "bound",
			want:     []byte("hello world"),
		},
		{
			name:     "empty body (boundary at start)",
			input:    strings.NewReader("\r\n--bound"),
			boundary: "bound",
			want:     []byte{},
		},
		{
			name: "boundary split across reads",
			input: io.MultiReader(
				strings.NewReader("hello"),
				strings.NewReader("\r\n--"),
				strings.NewReader("bound"),
			),
			boundary: "bound",
			want:     []byte("hello"),
		},
		{
			name:     "no boundary in stream returns all data",
			input:    strings.NewReader("just some data with no boundary at all"),
			boundary: "bound",
			want:     []byte("just some data with no boundary at all"),
		},
		{
			name:     "body with trailing content after boundary",
			input:    strings.NewReader("first part\r\n--sep\r\nsecond part"),
			boundary: "sep",
			want:     []byte("first part"),
		},
		{
			name: "binary data (all 256 byte values) before boundary",
			input: func() io.Reader {
				body := make([]byte, 256)
				for i := range body {
					body[i] = byte(i)
				}
				return bytes.NewReader(append(body, []byte("\r\n--b")...))
			}(),
			boundary: "b",
			want: func() []byte {
				b := make([]byte, 256)
				for i := range b {
					b[i] = byte(i)
				}
				return b
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := mime.NewBoundaryReader(tt.input, tt.boundary)
			got, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBoundaryReaderSmallBuffer(t *testing.T) {
	input := strings.NewReader("abc\r\n--x")
	r := mime.NewBoundaryReader(input, "x")

	var got []byte
	p := make([]byte, 1)
	for {
		n, err := r.Read(p)
		got = append(got, p[:n]...)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}
	assert.Equal(t, []byte("abc"), got)
}

func TestBoundaryReaderMultiplePartsSequential(t *testing.T) {
	// Simulate reading two parts from the same underlying reader
	// by verifying the first reader stops before the boundary marker.
	raw := "first\r\n--sep\r\nsecond"
	r := mime.NewBoundaryReader(strings.NewReader(raw), "sep")
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte("first"), got)
}
