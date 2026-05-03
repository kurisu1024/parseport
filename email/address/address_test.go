package address_test

import (
	"errors"
	"testing"

	"github.com/kurisu2024/parseport/email/address"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAddress(t *testing.T) {
	for _, tt := range []struct {
		name        string
		addr        string
		displayName string
		localpart   string
		domain      string
	}{
		// Plain addr-spec
		{
			name:      "plain_valid_address",
			addr:      "parseport@example.com",
			localpart: "parseport",
			domain:    "example.com",
		},
		{
			name:      "subdomain",
			addr:      "user@mail.example.co.uk",
			localpart: "user",
			domain:    "mail.example.co.uk",
		},
		{
			name:      "plus_tag",
			addr:      "user+tag@example.com",
			localpart: "user+tag",
			domain:    "example.com",
		},
		{
			name:      "all_atext",
			addr:      "!#$%&'*+-/=?^_`{|}~@x.y",
			localpart: "!#$%&'*+-/=?^_`{|}~",
			domain:    "x.y",
		},
		{
			name:      "single_char",
			addr:      "a@b.com",
			localpart: "a",
			domain:    "b.com",
		},

		// Quoted local-parts
		{
			name:      "quoted_local_with_space",
			addr:      `"hello world"@example.com`,
			localpart: "hello world",
			domain:    "example.com",
		},
		{
			name:      "quoted_local_escaped_quote",
			addr:      `"hello\"world"@example.com`,
			localpart: `hello"world`,
			domain:    "example.com",
		},
		{
			name:      "quoted_local_escaped_backslash",
			addr:      `"foo\\bar"@example.com`,
			localpart: `foo\bar`,
			domain:    "example.com",
		},

		// Domain literals
		{
			name:      "domain_literal_ipv4",
			addr:      "user@[192.0.2.1]",
			localpart: "user",
			domain:    "[192.0.2.1]",
		},
		{
			name:      "domain_literal_ipv6",
			addr:      "user@[IPv6:2001:db8::1]",
			localpart: "user",
			domain:    "[IPv6:2001:db8::1]",
		},

		// CFWS (comments stripped)
		{
			name:      "comment_before_at",
			addr:      "user(comment)@example.com",
			localpart: "user",
			domain:    "example.com",
		},
		{
			name:      "leading_whitespace",
			addr:      " user@example.com",
			localpart: "user",
			domain:    "example.com",
		},
		{
			name:      "trailing_whitespace",
			addr:      "user@example.com ",
			localpart: "user",
			domain:    "example.com",
		},

		// Name-addr (RFC 5322 mailbox)
		{
			name:      "angle_no_display_name",
			addr:      "<user@example.com>",
			localpart: "user",
			domain:    "example.com",
		},
		{
			name:        "display_name_two_atoms",
			addr:        "John Doe <john@example.com>",
			displayName: "John Doe",
			localpart:   "john",
			domain:      "example.com",
		},
		{
			name:        "display_name_quoted",
			addr:        `"John Doe" <john@example.com>`,
			displayName: "John Doe",
			localpart:   "john",
			domain:      "example.com",
		},
		{
			name:        "display_name_single_atom",
			addr:        "Alice <alice@example.com>",
			displayName: "Alice",
			localpart:   "alice",
			domain:      "example.com",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := address.ParseAddress(tt.addr)
			require.NoError(t, err)
			assert.Equal(t, tt.displayName, got.DisplayName)
			assert.Equal(t, tt.localpart, got.Localpart)
			assert.Equal(t, tt.domain, got.Domain)
		})
	}
}

func TestParseAddressErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		addr    string
		context string
	}{
		{name: "empty", addr: "", context: "mailbox"},
		{name: "at_only", addr: "@", context: "local-part"},
		{name: "leading_dot_local", addr: ".user@example.com", context: "local-part"},
		{name: "trailing_dot_local", addr: "user.@example.com", context: "dot-atom"},
		{name: "consecutive_dots_local", addr: "us..er@example.com", context: "dot-atom"},
		{name: "missing_domain", addr: "user@", context: "domain"},
		{name: "unclosed_quote", addr: `"user@example.com`, context: "quoted-string"},
		{name: "unclosed_bracket", addr: "user@[192.0.2.1", context: "domain-literal"},
		{name: "unclosed_angle", addr: "<user@example.com", context: "angle-addr"},
		{name: "unclosed_comment", addr: "(hello user@x.com", context: "CFWS comment"},
		{name: "trailing_garbage", addr: "user@x.com garbage"},
		{name: "double_at", addr: "user@@example.com", context: "domain"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := address.ParseAddress(tt.addr)
			require.Error(t, err)
			if tt.context != "" {
				var perr *address.ParseError
				require.True(t, errors.As(err, &perr), "expected *ParseError, got %T: %v", err, err)
				assert.Equal(t, tt.context, perr.Context, "unexpected ParseError.Context")
			}
		})
	}
}
