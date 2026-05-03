package address

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type tokenType int

const (
	tokenEOF          tokenType = iota
	tokenIllegal                // unexpected byte; Next returns *ParseError
	tokenAtom                   // atext+ per RFC 5321 §4.1.2
	tokenDot                    // "."
	tokenAt                     // "@"
	tokenQuotedString           // full "..." content, backslash-escapes decoded, quotes stripped
	tokenLAngle                 // "<"
	tokenRAngle                 // ">"
	tokenLBracket               // "["; val = dtext interior (closing "]" consumed internally)
)

type token struct {
	typ tokenType
	val string // empty for single-char punctuation tokens
	pos int    // byte offset of the first byte of this token
}

type lexer struct {
	r      *bufio.Reader
	pos    int    // running byte offset into the input
	peeked *token // non-nil when Peek has been called
}

func newLexer(r io.Reader) *lexer {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &lexer{r: br}
}

// Next returns the next meaningful token, skipping CFWS (comments and folding
// whitespace) transparently. On io.EOF it returns token{typ: tokenEOF}.
func (l *lexer) Next() (token, error) {
	if l.peeked != nil {
		t := *l.peeked
		l.peeked = nil
		return t, nil
	}
	return l.next()
}

// Peek returns the next token without consuming it.
func (l *lexer) Peek() (token, error) {
	if l.peeked != nil {
		return *l.peeked, nil
	}
	t, err := l.next()
	if err != nil {
		return t, err
	}
	l.peeked = &t
	return t, nil
}

func (l *lexer) next() (token, error) {
	if err := l.skipCFWS(); err != nil {
		return token{}, err
	}
	pos := l.pos
	b, err := l.r.ReadByte()
	if err == io.EOF {
		return token{typ: tokenEOF, pos: pos}, nil
	}
	if err != nil {
		return token{}, err
	}
	l.pos++

	switch {
	case b == '@':
		return token{typ: tokenAt, pos: pos}, nil
	case b == '.':
		return token{typ: tokenDot, pos: pos}, nil
	case b == '<':
		return token{typ: tokenLAngle, pos: pos}, nil
	case b == '>':
		return token{typ: tokenRAngle, pos: pos}, nil
	case b == '"':
		return l.scanQuotedString(pos)
	case b == '[':
		return l.scanDomainLiteral(pos)
	case isAtext(b):
		return l.scanAtom(b, pos)
	default:
		return token{typ: tokenIllegal, pos: pos}, &ParseError{
			Pos:     pos,
			Got:     b,
			Context: "input",
			Msg:     fmt.Sprintf("unexpected byte 0x%02X", b),
		}
	}
}

// skipCFWS consumes comments, horizontal whitespace, and folded whitespace
// (CRLF followed by at least one WSP, per RFC 5322 §3.2.2).
func (l *lexer) skipCFWS() error {
	for {
		b, err := l.r.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch {
		case b == ' ' || b == '\t':
			l.pos++
		case b == '\r':
			l.pos++
			// Expect \n for CRLF
			next, err := l.r.ReadByte()
			if err == io.EOF {
				return &ParseError{Pos: l.pos - 1, Context: "CFWS", Msg: "bare CR at end of input"}
			}
			if err != nil {
				return err
			}
			l.pos++
			if next != '\n' {
				return &ParseError{Pos: l.pos - 1, Got: next, Context: "CFWS", Msg: "CR not followed by LF"}
			}
			// CRLF must be followed by WSP for it to be folding whitespace
			wsp, err := l.r.ReadByte()
			if err == io.EOF {
				return &ParseError{Pos: l.pos, Context: "CFWS", Msg: "CRLF at end of input without folding WSP"}
			}
			if err != nil {
				return err
			}
			l.pos++
			if wsp != ' ' && wsp != '\t' {
				return &ParseError{Pos: l.pos - 1, Got: wsp, Context: "CFWS", Msg: "CRLF not followed by WSP (not folding whitespace)"}
			}
		case b == '(':
			l.pos++
			if err := l.skipComment(); err != nil {
				return err
			}
		default:
			// Not CFWS — put the byte back
			_ = l.r.UnreadByte()
			return nil
		}
	}
}

// skipComment reads through a parenthesized comment, handling nesting and
// quoted-pairs. The opening '(' has already been consumed.
func (l *lexer) skipComment() error {
	depth := 1
	for depth > 0 {
		b, err := l.r.ReadByte()
		if err == io.EOF {
			return &ParseError{Pos: l.pos, Context: "CFWS comment", Msg: "unexpected end of input in comment"}
		}
		if err != nil {
			return err
		}
		l.pos++
		switch b {
		case '(':
			depth++
		case ')':
			depth--
		case '\\':
			// quoted-pair inside comment: consume the next byte
			escaped, err := l.r.ReadByte()
			if err == io.EOF {
				return &ParseError{Pos: l.pos, Context: "CFWS comment", Msg: "unexpected end of input after backslash in comment"}
			}
			if err != nil {
				return err
			}
			l.pos++
			if !isVCHAR(escaped) && !isWSP(escaped) {
				return &ParseError{Pos: l.pos - 1, Got: escaped, Context: "CFWS comment", Msg: "invalid quoted-pair in comment"}
			}
		}
	}
	return nil
}

func (l *lexer) scanAtom(first byte, pos int) (token, error) {
	var b strings.Builder
	b.WriteByte(first)
	for {
		next, err := l.r.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return token{}, err
		}
		if !isAtext(next) {
			_ = l.r.UnreadByte()
			break
		}
		l.pos++
		b.WriteByte(next)
	}
	return token{typ: tokenAtom, val: b.String(), pos: pos}, nil
}

// scanQuotedString scans a quoted-string. The opening DQUOTE has already been
// consumed. The token val is the decoded content (escapes resolved, quotes stripped).
func (l *lexer) scanQuotedString(pos int) (token, error) {
	var b strings.Builder
	for {
		next, err := l.r.ReadByte()
		if err == io.EOF {
			return token{}, &ParseError{Pos: l.pos, Context: "quoted-string", Msg: "unexpected end of input in quoted-string"}
		}
		if err != nil {
			return token{}, err
		}
		l.pos++
		switch {
		case next == '"':
			return token{typ: tokenQuotedString, val: b.String(), pos: pos}, nil
		case next == '\\':
			escaped, err := l.r.ReadByte()
			if err == io.EOF {
				return token{}, &ParseError{Pos: l.pos, Context: "quoted-string", Msg: "unexpected end of input after backslash"}
			}
			if err != nil {
				return token{}, err
			}
			l.pos++
			if !isVCHAR(escaped) && !isWSP(escaped) {
				return token{}, &ParseError{Pos: l.pos - 1, Got: escaped, Context: "quoted-string", Msg: "invalid quoted-pair"}
			}
			b.WriteByte(escaped)
		case next == '\r' || next == '\n':
			return token{}, &ParseError{Pos: l.pos - 1, Got: next, Context: "quoted-string", Msg: "bare CRLF inside quoted-string"}
		case isQtext(next):
			b.WriteByte(next)
		default:
			return token{}, &ParseError{Pos: l.pos - 1, Got: next, Context: "quoted-string", Msg: fmt.Sprintf("invalid byte 0x%02X in quoted-string", next)}
		}
	}
}

// scanDomainLiteral scans a domain-literal. The opening '[' has already been
// consumed. The token val is the dtext interior; the closing ']' is consumed.
func (l *lexer) scanDomainLiteral(pos int) (token, error) {
	var b strings.Builder
	for {
		next, err := l.r.ReadByte()
		if err == io.EOF {
			return token{}, &ParseError{Pos: l.pos, Context: "domain-literal", Msg: "unexpected end of input in domain-literal"}
		}
		if err != nil {
			return token{}, err
		}
		l.pos++
		switch {
		case next == ']':
			return token{typ: tokenLBracket, val: b.String(), pos: pos}, nil
		case isDtext(next):
			b.WriteByte(next)
		default:
			return token{}, &ParseError{Pos: l.pos - 1, Got: next, Context: "domain-literal", Msg: fmt.Sprintf("invalid byte 0x%02X in domain-literal", next)}
		}
	}
}

// isAtext reports whether b is a valid atext character per RFC 5321 §4.1.2.
// Includes alpha, digit, and the 19 symbol chars; excludes . @ " [ ] < > ( ) \ : ; ,
func isAtext(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '!' || b == '#' || b == '$' || b == '%' || b == '&' ||
		b == '\'' || b == '*' || b == '+' || b == '-' || b == '/' ||
		b == '=' || b == '?' || b == '^' || b == '_' || b == '`' ||
		b == '{' || b == '|' || b == '}' || b == '~'
}

// isQtext reports whether b is valid inside a quoted-string (not " or \).
func isQtext(b byte) bool {
	return b == 0x21 || (b >= 0x23 && b <= 0x5B) || (b >= 0x5D && b <= 0x7E) || isWSP(b)
}

// isDtext reports whether b is valid inside a domain-literal (not [ ] \).
func isDtext(b byte) bool {
	return (b >= 0x21 && b <= 0x5A) || (b >= 0x5E && b <= 0x7E)
}

// isWSP reports whether b is horizontal whitespace.
func isWSP(b byte) bool { return b == ' ' || b == '\t' }

// isVCHAR reports whether b is a visible (printable) ASCII character.
func isVCHAR(b byte) bool { return b >= 0x21 && b <= 0x7E }
