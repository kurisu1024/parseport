# parseport

A performance-optimized, streaming parser toolkit for Go.

**Repo:** `github.com/kurisu2024/parseport`

---

## Design Philosophy

### Streaming First
All parsers are built on `io.Reader` and stream data incrementally. No full
buffering into memory. This makes parseport suitable for large payloads, network
streams, and high-throughput applications.

### Layered Reader Pipeline
Decoding is handled by composing thin `io.Reader` wrappers into a pipeline.
Each layer has one responsibility — transfer decoding, charset decoding,
word decoding, etc. Layers are independently testable and reusable.

```
io.Reader (raw source)
    └── HeaderParser        ← streams + parses headers, applies options
            └── BodyDecoder ← picks up stream after \r\n\r\n boundary
                    └── TransferDecoder  (e.g. base64, quoted-printable)
                            └── CharsetDecoder  (e.g. UTF-8, Latin-1)
```

### Pluggable Options via `func(io.Reader) io.Reader`
Rather than configuration structs or booleans, options are reader transforms.
This keeps packages decoupled and allows third-party extensions without
touching parseport internals.

```go
// Option is a reader transform — wraps and returns a new io.Reader
type Option func(io.Reader) io.Reader

// NewHeaderParser chains all options over the source reader
func NewHeaderParser(r io.Reader, opts ...Option) *HeaderParser {
    for _, opt := range opts {
        r = opt(r)
    }
    // store r ...
}
```

### Package Independence
Each package is independently importable. Dependency arrows only point upward —
lower-level packages never import higher-level ones. No circular dependencies.

```
header/       ← no dependencies on other parseport packages
mime/         ← imports header/
email/address ← no dependencies on other parseport packages
email/message ← imports header/, mime/
http/         ← imports header/ only, no mime/ dependency
uri/          ← standalone
```

---

## Package Structure

```
parseport/
  header/            ← standalone streaming header parser, format-agnostic
  mime/              ← MIME parser, imports header/
  email/
    address/         ← RFC 5321/5322 email address parser, standalone
    message/         ← full email parser, imports header/ and mime/
  http/              ← HTTP header parser, imports header/ only
  uri/               ← URI parser (RFC 3986), standalone
  csv/               ← CSV parser (RFC 4180), standalone
```

---

## API Design

### Streaming API — `Decoder` naming convention
Follows Go stdlib convention: streaming APIs use `NewDecoder(r io.Reader)`,
in-memory convenience wrappers use `Parse*(s string)`.

```go
// Streaming (primary API)
dec := mime.NewDecoder(r)
hdr, err := dec.Header()
body := dec.BodyReader()

// Convenience wrapper (thin layer over streaming core)
hdr, err := mime.ParseHeader(s)
```

### Header Options — Pluggable Reader Transforms
`header/` is format-agnostic. Format-specific behavior is injected via options
defined in higher-level packages. `header/` has no knowledge of MIME, HTTP,
or any other format.

```go
// mime/ defines its own option — header/ never imports mime/
dec := header.NewHeaderParser(r,
    mime.WordDecoderOption,   // RFC 2047 encoded-word decoding
    mime.CharsetOption,       // charset transform via golang.org/x/text
)

// http/ defines its own option — completely independent of mime/
dec := header.NewHeaderParser(r,
    http.CanonicalizeOption,  // HTTP canonical header form
)

// third-party options work the same way — no changes to parseport needed
dec := header.NewHeaderParser(r,
    mypackage.CustomSanitizerOption,
)
```

### BodyDecoder — Self-Configuring from Header
`BodyDecoder` takes a parsed `*Header` and wires its own reader pipeline
automatically based on `Content-Transfer-Encoding` and `charset`.

```go
body := mime.NewBodyDecoder(r, hdr)
// internally layers: TransferDecoder → CharsetDecoder → io.Reader
// caller just reads through transparently
```

### Strictness Modes
Parsers support both strict (spec-compliant, rejects invalid input) and lenient
(real-world tolerant) modes, selectable via an option:

```go
dec := header.NewHeaderParser(r, header.WithLenient)
```

---


## TODO

### Project Setup
- [ ] Initialize Go module as `github.com/kurisu2024/parseport`
- [ ] Scaffold package directories per structure above
- [ ] Add `go.mod`, `go.sum`, `.gitignore`, `LICENSE`
- [ ] Add `golang.org/x/text` dependency for charset transform support

---

### `header/` — Streaming Header Parser
- [ ] Define `type Option func(io.Reader) io.Reader`
- [ ] Implement `NewHeaderParser(r io.Reader, opts ...Option) *HeaderParser`
- [ ] Chain options over source reader in `NewHeaderParser`
- [ ] Stream and parse headers line by line using `bufio.Reader`
- [ ] Handle line folding (multi-line header values)
- [ ] Support multi-value fields
- [ ] Implement `BodyReader() io.Reader` — hands off stream cleanly after `\r\n\r\n`, no over-buffering
- [ ] Add `WithLenient` / `WithStrict` mode option
- [ ] Add `WithMaxHeaderSize(n int)` option for defense against malicious input
- [ ] No imports from `mime/`, `email/`, or `http/` — keep standalone

---

### `mime/` — MIME Parser
#### Header Options (defined in mime/, consumed by header/)
- [ ] Implement `WordDecoderOption` — RFC 2047 encoded-word decoding as `Option`
- [ ] Implement `CharsetOption` — charset transform via `golang.org/x/text/transform` as `Option`

#### Header Decoding
- [ ] Implement `NewDecoder(r io.Reader) *Decoder`
- [ ] Parse `Content-Type`: media type, subtype, parameters
- [ ] Parse `Content-Disposition` and parameters (e.g. `filename`)
- [ ] Parse `charset` from header parameters
- [ ] Expose `Decoder.Header() (*Header, error)`
- [ ] Expose `Decoder.BodyReader() io.Reader`

#### Body Decoding
- [ ] Implement `NewBodyDecoder(r io.Reader, hdr *Header) *BodyDecoder`
- [ ] Auto-select transfer decoding layer from `Content-Transfer-Encoding`
- [ ] Wire `encoding/base64` reader for `base64`
- [ ] Wire `mime/quotedprintable` reader for `quoted-printable`
- [ ] Auto-select charset layer from parsed `charset` via `golang.org/x/text`
- [ ] Implement `BodyDecoder.Read(p []byte)` — fully layered, transparent to caller

#### General MIME
- [ ] IANA media type validation
- [ ] Structured syntax suffix support (`+json`, `+xml`, `+zip` per RFC 6838)
- [ ] `multipart/*` boundary streaming and parsing

---

### `email/address/` — Email Address Parser
- [ ] RFC 5321/5322 compliant address parsing
- [ ] Support single address, address list, and group syntax
- [ ] Streaming reader-based core API
- [ ] Convenience wrapper: `address.Parse(s string) (*Address, error)`
- [ ] No dependencies on `mime/` or `header/`

---

### `email/message/` — Full Email Message Parser
- [ ] Compose `header.NewHeaderParser` with `mime/` options
- [ ] Full body decoding via `mime.NewBodyDecoder`
- [ ] Support nested `message/rfc822` (recursive emails within emails)

---

### `http/` — HTTP Header Parser
- [ ] Implement `CanonicalizeOption` as `header.Option`
- [ ] Use `header.NewHeaderParser` with HTTP-specific options
- [ ] No dependency on `mime/`

---

### `uri/` — URI Parser
- [ ] RFC 3986 compliant URI parsing
- [ ] Standalone, no dependencies on other parseport packages

---

### `csv/` — CSV Parser
- [ ] RFC 4180 compliant CSV parsing
- [ ] Streaming reader-based API
- [ ] Standalone

---

### API Consistency
- [ ] `NewDecoder(r io.Reader)` convention on all streaming parsers
- [ ] `Parse*(s string)` convenience wrappers on all parsers
- [ ] Caller-provided buffer support on hot paths
- [ ] All packages independently importable

---

### Testing & Quality
- [ ] Unit tests per package
- [ ] Fuzz testing for all parsers
- [ ] Benchmarks for hot paths
- [ ] Test `email/message` against real-world email corpus for lenient mode
- [ ] Test option chaining and composition

---

### Documentation
- [ ] GoDoc on all exported types and functions
- [ ] Usage examples per package
- [ ] Document strictness and error handling behavior per parser
- [ ] Expand README as packages ship
