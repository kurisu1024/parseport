package address

import (
	"io"
	"strings"
)

type tokenReader interface {
	Next() (token, error)
	Peek() (token, error)
}

type parser struct {
	lex tokenReader
}

func newParser(r io.Reader) *parser {
	return &parser{lex: newLexer(r)}
}

// parse is the top-level entry point. It parses one mailbox and then requires EOF.
func (p *parser) parse() (Address, error) {
	addr, err := p.parseMailbox()
	if err != nil {
		return Address{}, err
	}
	tok, err := p.lex.Next()
	if err != nil {
		return Address{}, err
	}
	if tok.typ != tokenEOF {
		return Address{}, &ParseError{
			Pos:     tok.pos,
			Got:     0,
			Context: "mailbox",
			Msg:     "unexpected content after address",
		}
	}
	return addr, nil
}

// parseMailbox handles: mailbox = name-addr / addr-spec
//
// Disambiguation: collect leading atom/quoted-string tokens until we see
// tokenAt (addr-spec path) or tokenLAngle (name-addr path).
func (p *parser) parseMailbox() (Address, error) {
	var pre []token
	for {
		tok, err := p.lex.Peek()
		if err != nil {
			return Address{}, err
		}
		switch tok.typ {
		case tokenEOF:
			return Address{}, &ParseError{Pos: tok.pos, Context: "mailbox", Msg: "unexpected end of input"}
		case tokenAt:
			// pre tokens are the local-part; delegate to parseAddrSpec with pre
			return p.parseAddrSpecFromTokens(pre)
		case tokenLAngle:
			// pre tokens are the display name
			return p.parseNameAddr(pre)
		case tokenAtom, tokenQuotedString, tokenDot:
			// consume and accumulate — we'll decide once we see @ or <
			_, _ = p.lex.Next()
			pre = append(pre, tok)
		case tokenIllegal:
			_, _ = p.lex.Next()
			return Address{}, &ParseError{Pos: tok.pos, Got: tok.val[0], Context: "mailbox", Msg: "unexpected character"}
		default:
			return Address{}, &ParseError{Pos: tok.pos, Context: "mailbox", Msg: "expected address"}
		}
	}
}

// parseNameAddr parses: [display-name] "<" addr-spec ">"
// pre holds the already-consumed display-name tokens.
func (p *parser) parseNameAddr(pre []token) (Address, error) {
	displayName := buildDisplayName(pre)

	// Consume the '<'
	langle, err := p.lex.Next()
	if err != nil {
		return Address{}, err
	}
	if langle.typ != tokenLAngle {
		return Address{}, &ParseError{Pos: langle.pos, Context: "angle-addr", Msg: "expected '<'"}
	}

	addr, err := p.parseAddrSpec()
	if err != nil {
		return Address{}, err
	}

	rangle, err := p.lex.Next()
	if err != nil {
		return Address{}, err
	}
	if rangle.typ != tokenRAngle {
		return Address{}, &ParseError{Pos: rangle.pos, Context: "angle-addr", Msg: "expected '>' after addr-spec"}
	}

	addr.DisplayName = displayName
	return addr, nil
}

// parseAddrSpec parses: local-part "@" domain
func (p *parser) parseAddrSpec() (Address, error) {
	lp, err := p.parseLocalPart()
	if err != nil {
		return Address{}, err
	}

	at, err := p.lex.Next()
	if err != nil {
		return Address{}, err
	}
	if at.typ != tokenAt {
		return Address{}, &ParseError{Pos: at.pos, Context: "addr-spec", Msg: "expected '@'"}
	}

	domain, err := p.parseDomain()
	if err != nil {
		return Address{}, err
	}

	return Address{Localpart: lp, Domain: domain}, nil
}

// parseAddrSpecFromTokens handles the addr-spec path when the local-part
// tokens have already been consumed during disambiguation in parseMailbox.
func (p *parser) parseAddrSpecFromTokens(pre []token) (Address, error) {
	if len(pre) == 0 {
		tok, err := p.lex.Peek()
		if err != nil {
			return Address{}, err
		}
		return Address{}, &ParseError{Pos: tok.pos, Context: "local-part", Msg: "empty local-part"}
	}

	lp, err := p.buildLocalPartFromTokens(pre)
	if err != nil {
		return Address{}, err
	}

	at, err := p.lex.Next()
	if err != nil {
		return Address{}, err
	}
	if at.typ != tokenAt {
		return Address{}, &ParseError{Pos: at.pos, Context: "addr-spec", Msg: "expected '@'"}
	}

	domain, err := p.parseDomain()
	if err != nil {
		return Address{}, err
	}

	return Address{Localpart: lp, Domain: domain}, nil
}

// parseLocalPart parses: dot-atom / quoted-string
func (p *parser) parseLocalPart() (string, error) {
	tok, err := p.lex.Next()
	if err != nil {
		return "", err
	}
	switch tok.typ {
	case tokenQuotedString:
		return tok.val, nil
	case tokenAtom:
		return p.parseDotAtom(tok)
	case tokenDot:
		return "", &ParseError{Pos: tok.pos, Context: "local-part", Msg: "local-part may not begin with a dot"}
	case tokenEOF:
		return "", &ParseError{Pos: tok.pos, Context: "local-part", Msg: "unexpected end of input"}
	default:
		return "", &ParseError{Pos: tok.pos, Context: "local-part", Msg: "expected atom or quoted-string"}
	}
}

// buildLocalPartFromTokens reconstructs the local-part string from the tokens
// already consumed during the mailbox disambiguation pass.
func (p *parser) buildLocalPartFromTokens(toks []token) (string, error) {
	if len(toks) == 0 {
		return "", &ParseError{Context: "local-part", Msg: "empty local-part"}
	}

	// A single quoted-string is a complete local-part.
	if len(toks) == 1 && toks[0].typ == tokenQuotedString {
		return toks[0].val, nil
	}

	// Validate and join dot-atom tokens.
	if toks[0].typ == tokenDot {
		return "", &ParseError{Pos: toks[0].pos, Context: "local-part", Msg: "local-part may not begin with a dot"}
	}
	if toks[len(toks)-1].typ == tokenDot {
		return "", &ParseError{Pos: toks[len(toks)-1].pos, Context: "dot-atom", Msg: "dot-atom may not end with a dot"}
	}

	var sb strings.Builder
	for i, tok := range toks {
		switch tok.typ {
		case tokenAtom:
			sb.WriteString(tok.val)
		case tokenDot:
			if i+1 < len(toks) && toks[i+1].typ == tokenDot {
				return "", &ParseError{Pos: tok.pos, Context: "dot-atom", Msg: "dot-atom may not contain consecutive dots"}
			}
			sb.WriteByte('.')
		default:
			return "", &ParseError{Pos: tok.pos, Context: "local-part", Msg: "unexpected token in local-part"}
		}
	}
	return sb.String(), nil
}

// parseDotAtom builds a dot-atom string from an initial atom token. It continues
// consuming dot+atom pairs while they are available.
func (p *parser) parseDotAtom(first token) (string, error) {
	var sb strings.Builder
	sb.WriteString(first.val)

	for {
		peek, err := p.lex.Peek()
		if err != nil {
			return "", err
		}
		if peek.typ != tokenDot {
			break
		}
		// Consume the dot
		dotTok, _ := p.lex.Next()

		// Must be followed by an atom
		next, err := p.lex.Peek()
		if err != nil {
			return "", err
		}
		if next.typ != tokenAtom {
			return "", &ParseError{Pos: dotTok.pos, Context: "dot-atom", Msg: "dot-atom may not end with a dot"}
		}
		_, _ = p.lex.Next()
		sb.WriteByte('.')
		sb.WriteString(next.val)
	}
	return sb.String(), nil
}

// parseDomain parses: dot-atom / domain-literal
func (p *parser) parseDomain() (string, error) {
	tok, err := p.lex.Next()
	if err != nil {
		return "", err
	}
	switch tok.typ {
	case tokenLBracket:
		return "[" + tok.val + "]", nil
	case tokenAtom:
		return p.parseDotAtom(tok)
	case tokenEOF:
		return "", &ParseError{Pos: tok.pos, Context: "domain", Msg: "unexpected end of input"}
	default:
		return "", &ParseError{Pos: tok.pos, Context: "domain", Msg: "expected domain name or domain-literal"}
	}
}

// buildDisplayName joins display-name tokens into a single string. Adjacent
// atoms are separated by a single space; quoted-string decoded values are
// used without surrounding quotes.
func buildDisplayName(toks []token) string {
	if len(toks) == 0 {
		return ""
	}
	var parts []string
	for _, tok := range toks {
		switch tok.typ {
		case tokenAtom:
			parts = append(parts, tok.val)
		case tokenQuotedString:
			parts = append(parts, tok.val)
		case tokenDot:
			// dots between display-name atoms are unusual but preserved
			parts = append(parts, ".")
		}
	}
	return strings.Join(parts, " ")
}

func newConcurrentParser(r io.Reader) (Address, error) {
	return (&parser{lex: newChanLexer(r)}).parse()
}
