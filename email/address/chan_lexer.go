package address

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type stateFn func(*chanLexer) stateFn

type chanLexer struct {
	r      *bufio.Reader
	pos    int
	tokens chan token
	lexErr *ParseError // set by lexer goroutine before sending tokenIllegal
	peeked *token      // single-slot Peek buffer, consumer side only
}

func newChanLexer(r io.Reader) *chanLexer {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, 256)
	}
	cl := &chanLexer{
		r:      br,
		tokens: make(chan token, 16),
	}
	go cl.run()
	return cl
}

func (cl *chanLexer) run() {
	for state := lexStart; state != nil; {
		state = state(cl)
	}
	close(cl.tokens)
}

// Next returns the next token. Satisfies tokenReader.
func (cl *chanLexer) Next() (token, error) {
	if cl.peeked != nil {
		t := *cl.peeked
		cl.peeked = nil
		return t, nil
	}
	t, ok := <-cl.tokens
	if !ok {
		return token{typ: tokenEOF}, nil
	}
	if t.typ == tokenIllegal {
		// cl.lexErr was set before the token was sent; channel receive is happens-before
		return t, cl.lexErr
	}
	return t, nil
}

// Peek returns the next token without consuming it. Satisfies tokenReader.
func (cl *chanLexer) Peek() (token, error) {
	if cl.peeked != nil {
		return *cl.peeked, nil
	}
	t, err := cl.Next()
	cl.peeked = &t
	return t, err
}

// emit sends a token to the parser.
func (cl *chanLexer) emit(t token) { cl.tokens <- t }

// fail records a ParseError and emits tokenIllegal, ending the lexer.
func (cl *chanLexer) fail(pos int, got byte, context, msg string) stateFn {
	cl.lexErr = &ParseError{Pos: pos, Got: got, Context: context, Msg: msg}
	cl.tokens <- token{typ: tokenIllegal, pos: pos}
	return nil
}

// readByte reads the next byte, advancing pos.
func (cl *chanLexer) readByte() (byte, error) {
	b, err := cl.r.ReadByte()
	if err == nil {
		cl.pos++
	}
	return b, err
}

func (cl *chanLexer) unreadByte() { _ = cl.r.UnreadByte() }

// skipCFWS consumes whitespace and parenthesized comments.
func (cl *chanLexer) skipCFWS() error {
	for {
		b, err := cl.r.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch {
		case b == ' ' || b == '\t':
			cl.pos++
		case b == '\r':
			cl.pos++
			next, err := cl.r.ReadByte()
			if err == io.EOF {
				return &ParseError{Pos: cl.pos - 1, Context: "CFWS", Msg: "bare CR at end of input"}
			}
			if err != nil {
				return err
			}
			cl.pos++
			if next != '\n' {
				return &ParseError{Pos: cl.pos - 1, Got: next, Context: "CFWS", Msg: "CR not followed by LF"}
			}
			wsp, err := cl.r.ReadByte()
			if err == io.EOF {
				return &ParseError{Pos: cl.pos, Context: "CFWS", Msg: "CRLF at end of input without folding WSP"}
			}
			if err != nil {
				return err
			}
			cl.pos++
			if wsp != ' ' && wsp != '\t' {
				return &ParseError{Pos: cl.pos - 1, Got: wsp, Context: "CFWS", Msg: "CRLF not followed by WSP (not folding whitespace)"}
			}
		case b == '(':
			cl.pos++
			if err := cl.skipComment(); err != nil {
				return err
			}
		default:
			_ = cl.r.UnreadByte()
			return nil
		}
	}
}

func (cl *chanLexer) skipComment() error {
	depth := 1
	for depth > 0 {
		b, err := cl.r.ReadByte()
		if err == io.EOF {
			return &ParseError{Pos: cl.pos, Context: "CFWS comment", Msg: "unexpected end of input in comment"}
		}
		if err != nil {
			return err
		}
		cl.pos++
		switch b {
		case '(':
			depth++
		case ')':
			depth--
		case '\\':
			escaped, err := cl.r.ReadByte()
			if err == io.EOF {
				return &ParseError{Pos: cl.pos, Context: "CFWS comment", Msg: "unexpected end of input after backslash in comment"}
			}
			if err != nil {
				return err
			}
			cl.pos++
			if !isVCHAR(escaped) && !isWSP(escaped) {
				return &ParseError{Pos: cl.pos - 1, Got: escaped, Context: "CFWS comment", Msg: "invalid quoted-pair in comment"}
			}
		}
	}
	return nil
}

// --- State functions ---

func lexStart(l *chanLexer) stateFn {
	if err := l.skipCFWS(); err != nil {
		if pe, ok := err.(*ParseError); ok {
			l.lexErr = pe
			l.tokens <- token{typ: tokenIllegal, pos: pe.Pos}
			return nil
		}
		return nil
	}

	pos := l.pos
	b, err := l.r.ReadByte()
	if err == io.EOF {
		l.emit(token{typ: tokenEOF, pos: pos})
		return nil
	}
	if err != nil {
		return nil
	}
	l.pos++

	switch {
	case b == '@':
		l.emit(token{typ: tokenAt, pos: pos})
		return lexStart
	case b == '.':
		l.emit(token{typ: tokenDot, pos: pos})
		return lexStart
	case b == '<':
		l.emit(token{typ: tokenLAngle, pos: pos})
		return lexStart
	case b == '>':
		l.emit(token{typ: tokenRAngle, pos: pos})
		return lexStart
	case b == '"':
		return func(l *chanLexer) stateFn { return lexQuotedString(l, pos) }
	case b == '[':
		return func(l *chanLexer) stateFn { return lexDomainLiteral(l, pos) }
	case isAtext(b):
		return func(l *chanLexer) stateFn { return lexAtom(l, b, pos) }
	default:
		return l.fail(pos, b, "input", fmt.Sprintf("unexpected byte 0x%02X", b))
	}
}

func lexAtom(l *chanLexer, first byte, pos int) stateFn {
	var sb strings.Builder
	sb.WriteByte(first)
	for {
		b, err := l.r.ReadByte()
		if err != nil {
			break
		}
		if !isAtext(b) {
			_ = l.r.UnreadByte()
			break
		}
		l.pos++
		sb.WriteByte(b)
	}
	l.emit(token{typ: tokenAtom, val: sb.String(), pos: pos})
	return lexStart
}

func lexQuotedString(l *chanLexer, pos int) stateFn {
	var sb strings.Builder
	for {
		b, err := l.r.ReadByte()
		if err == io.EOF {
			return l.fail(pos, 0, "quoted-string", "unexpected end of input in quoted-string")
		}
		if err != nil {
			return nil
		}
		l.pos++
		switch {
		case b == '"':
			l.emit(token{typ: tokenQuotedString, val: sb.String(), pos: pos})
			return lexStart
		case b == '\\':
			escaped, err := l.r.ReadByte()
			if err == io.EOF {
				return l.fail(l.pos, 0, "quoted-string", "unexpected end of input after backslash")
			}
			if err != nil {
				return nil
			}
			l.pos++
			if !isVCHAR(escaped) && !isWSP(escaped) {
				return l.fail(l.pos-1, escaped, "quoted-string", "invalid quoted-pair")
			}
			sb.WriteByte(escaped)
		case b == '\r' || b == '\n':
			return l.fail(l.pos-1, b, "quoted-string", "bare CRLF inside quoted-string")
		case isQtext(b):
			sb.WriteByte(b)
		default:
			return l.fail(l.pos-1, b, "quoted-string", fmt.Sprintf("invalid byte 0x%02X in quoted-string", b))
		}
	}
}

func lexDomainLiteral(l *chanLexer, pos int) stateFn {
	var sb strings.Builder
	for {
		b, err := l.r.ReadByte()
		if err == io.EOF {
			return l.fail(pos, 0, "domain-literal", "unexpected end of input in domain-literal")
		}
		if err != nil {
			return nil
		}
		l.pos++
		switch {
		case b == ']':
			l.emit(token{typ: tokenLBracket, val: sb.String(), pos: pos})
			return lexStart
		case isDtext(b):
			sb.WriteByte(b)
		default:
			return l.fail(l.pos-1, b, "domain-literal", fmt.Sprintf("invalid byte 0x%02X in domain-literal", b))
		}
	}
}
