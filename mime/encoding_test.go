package mime_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	mime "github.com/kurisu2024/parseport/mime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCharsetDecoderUTF8(t *testing.T) {
	t.Run("ascii passthrough", func(t *testing.T) {
		r, err := mime.NewCharsetDecoder(mime.UTF8, strings.NewReader("hello"))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), got)
	})

	t.Run("multibyte passthrough", func(t *testing.T) {
		input := "こんにちは"
		r, err := mime.NewCharsetDecoder(mime.UTF8, strings.NewReader(input))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, []byte(input), got)
	})

	t.Run("empty input", func(t *testing.T) {
		r, err := mime.NewCharsetDecoder(mime.UTF8, bytes.NewReader(nil))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestNewCharsetDecoderUnknown(t *testing.T) {
	_, err := mime.NewCharsetDecoder("ascii", strings.NewReader("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ascii")
}

func TestNewCharsetDecoderLatin1(t *testing.T) {
	t.Run("ascii range passes through unchanged", func(t *testing.T) {
		input := make([]byte, 128)
		for i := range input {
			input[i] = byte(i)
		}
		r, err := mime.NewCharsetDecoder(mime.LATIN1, bytes.NewReader(input))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, input, got)
	})

	t.Run("copyright sign 0xA9 encodes to 0xC2 0xA9", func(t *testing.T) {
		r, err := mime.NewCharsetDecoder(mime.LATIN1, bytes.NewReader([]byte{0xA9}))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, []byte{0xC2, 0xA9}, got)
	})

	t.Run("all extended bytes round-trip to correct runes", func(t *testing.T) {
		input := make([]byte, 128)
		for i := range input {
			input[i] = byte(0x80 + i)
		}
		r, err := mime.NewCharsetDecoder(mime.LATIN1, bytes.NewReader(input))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		require.True(t, utf8.Valid(got), "output must be valid UTF-8")
		runes := []rune(string(got))
		require.Len(t, runes, 128)
		for i, r := range runes {
			assert.Equal(t, rune(0x80+i), r, "rune at index %d", i)
		}
	})

	t.Run("small buffer exercises overflow path", func(t *testing.T) {
		// 0xE9 = é = U+00E9 = UTF-8: 0xC3 0xA9
		r, err := mime.NewCharsetDecoder(mime.LATIN1, bytes.NewReader([]byte{0xE9}))
		require.NoError(t, err)

		p1 := make([]byte, 1)
		n1, err1 := r.Read(p1)
		require.NoError(t, err1)
		assert.Equal(t, 1, n1)
		assert.Equal(t, byte(0xC3), p1[0])

		p2 := make([]byte, 1)
		n2, _ := r.Read(p2)
		assert.Equal(t, 1, n2)
		assert.Equal(t, byte(0xA9), p2[0])
	})

	t.Run("empty input", func(t *testing.T) {
		r, err := mime.NewCharsetDecoder(mime.LATIN1, bytes.NewReader(nil))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestNewCharsetDecoderUTF16(t *testing.T) {
	encodeUTF16BE := func(s string) []byte {
		var buf bytes.Buffer
		for _, r := range s {
			buf.WriteByte(byte(r >> 8))
			buf.WriteByte(byte(r & 0xFF))
		}
		return buf.Bytes()
	}
	encodeUTF16LE := func(s string) []byte {
		var buf bytes.Buffer
		for _, r := range s {
			buf.WriteByte(byte(r & 0xFF))
			buf.WriteByte(byte(r >> 8))
		}
		return buf.Bytes()
	}

	t.Run("big-endian with BOM", func(t *testing.T) {
		var buf bytes.Buffer
		buf.Write([]byte{0xFE, 0xFF}) // BOM BE
		buf.Write(encodeUTF16BE("Hello"))
		r, err := mime.NewCharsetDecoder(mime.UTF16, &buf)
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, []byte("Hello"), got)
	})

	t.Run("little-endian with BOM", func(t *testing.T) {
		var buf bytes.Buffer
		buf.Write([]byte{0xFF, 0xFE}) // BOM LE
		buf.Write(encodeUTF16LE("Hello"))
		r, err := mime.NewCharsetDecoder(mime.UTF16, &buf)
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, []byte("Hello"), got)
	})

	t.Run("no BOM defaults to big-endian", func(t *testing.T) {
		r, err := mime.NewCharsetDecoder(mime.UTF16, bytes.NewReader(encodeUTF16BE("Hi")))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, []byte("Hi"), got)
	})

	t.Run("surrogate pair encodes emoji U+1F600", func(t *testing.T) {
		// U+1F600 😀 → high surrogate 0xD83D, low surrogate 0xDE00
		var buf bytes.Buffer
		buf.Write([]byte{0xFE, 0xFF}) // BOM BE
		buf.Write([]byte{0xD8, 0x3D}) // high surrogate
		buf.Write([]byte{0xDE, 0x00}) // low surrogate
		r, err := mime.NewCharsetDecoder(mime.UTF16, &buf)
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, []byte("😀"), got)
	})

	t.Run("empty input", func(t *testing.T) {
		r, err := mime.NewCharsetDecoder(mime.UTF16, bytes.NewReader(nil))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("invalid stray low surrogate replaced with U+FFFD", func(t *testing.T) {
		// x/text replaces invalid surrogates with the Unicode replacement character
		var buf bytes.Buffer
		buf.Write([]byte{0xFE, 0xFF}) // BOM BE
		buf.Write([]byte{0xDC, 0x00}) // stray low surrogate 0xDC00
		r, err := mime.NewCharsetDecoder(mime.UTF16, &buf)
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, []byte("\xef\xbf\xbd"), got) // U+FFFD in UTF-8
	})
}
