package header

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []Header
		wantErr bool
	}{
		{
			name:  "single header",
			input: "Subject: Hello\r\n",
			want: []Header{
				{name: "Subject", value: "Hello", raw: []byte("Subject: Hello\r\n")},
			},
		},
		{
			name:  "name normalization",
			input: "content-type: text/plain\r\n",
			want: []Header{
				{name: "Content-Type", value: "text/plain", raw: []byte("content-type: text/plain\r\n")},
			},
		},
		{
			name:  "multiple headers",
			input: "From: Alice\r\nTo: Bob\r\n",
			want: []Header{
				{name: "From", value: "Alice", raw: []byte("From: Alice\r\n")},
				{name: "To", value: "Bob", raw: []byte("To: Bob\r\n")},
			},
		},
		{
			name:  "stops at empty line",
			input: "From: Alice\r\nTo: Bob\r\n\r\nbody content",
			want: []Header{
				{name: "From", value: "Alice", raw: []byte("From: Alice\r\n")},
				{name: "To", value: "Bob", raw: []byte("To: Bob\r\n")},
			},
		},
		{
			name:  "folded header with space",
			input: "Subject: This is\r\n a long subject\r\n",
			want: []Header{
				{
					name:  "Subject",
					value: "This is a long subject",
					raw:   []byte("Subject: This is\r\n a long subject\r\n"),
				},
			},
		},
		{
			name:  "folded header with tab",
			input: "Subject: This is\r\n\ta long subject\r\n",
			want: []Header{
				{
					name:  "Subject",
					value: "This is\ta long subject",
					raw:   []byte("Subject: This is\r\n\ta long subject\r\n"),
				},
			},
		},
		{
			name:  "multiple folds",
			input: "Received: from a\r\n by b\r\n for c\r\n",
			want: []Header{
				{
					name:  "Received",
					value: "from a by b for c",
					raw:   []byte("Received: from a\r\n by b\r\n for c\r\n"),
				},
			},
		},
		{
			name:  "value leading whitespace stripped",
			input: "Subject:   spaces\r\n",
			want: []Header{
				{name: "Subject", value: "spaces", raw: []byte("Subject:   spaces\r\n")},
			},
		},
		{
			name:  "raw preserves exact bytes",
			input: "X-Custom-Header: value\r\n",
			want: []Header{
				{name: "X-Custom-Header", value: "value", raw: []byte("X-Custom-Header: value\r\n")},
			},
		},
		{
			name:  "LF-only line endings",
			input: "Subject: Hello\nFrom: Bob\n",
			want: []Header{
				{name: "Subject", value: "Hello", raw: []byte("Subject: Hello\n")},
				{name: "From", value: "Bob", raw: []byte("From: Bob\n")},
			},
		},
		{
			name:    "missing colon",
			input:   "BadHeader\r\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHeaders(strings.NewReader(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, len(tt.want))
			for i, h := range got {
				assert.Equal(t, tt.want[i].name, h.name, "name[%d]", i)
				assert.Equal(t, tt.want[i].value, h.value, "value[%d]", i)
				assert.Equal(t, tt.want[i].raw, h.raw, "raw[%d]", i)
			}
		})
	}
}
