## Design Decisions

### Email Address Lexer: Iterator vs Channel-Based

`email/address/` ships two lexer implementations, benchmarked head-to-head on identical complex inputs (long dot-atoms, quoted display names with escapes, nested CFWS comments, IPv6 domain literals):

| Variant | Time | Heap | Allocations |
|---|---|---|---|
| **Sequential iterator** (`ReadAddress`) | **~1.5 µs/op** | ~5,400 B/op | ~42 allocs/op |
| Channel-based, Pike-style (`ReadAddressOptimized`) | ~2,600 ns/op | ~2,450 B/op | ~54 allocs/op |

#### What the numbers mean

**Time — sequential wins by 1.7×.**
The channel lexer runs the state machine in a separate goroutine and emits tokens through a buffered channel. For email addresses (~5–15 tokens, ~1–3 µs total work), the goroutine spawn and channel send/receive synchronization cost dominates the work itself. Inputs with more tokens (heavy CFWS, deep dot-atoms) show the worst ratios — every token is one channel hop.

**Allocations — sequential wins by ~12 fewer per op.**
The channel implementation pays for goroutine stack setup, the `chan token` buffer, and closure allocations for state functions that need to carry context between invocations (`lexAtom`, `lexQuotedString`, etc.). More allocations means more GC pressure under sustained load.

**Heap bytes — channel uses ~55% less, but it's not a real win.**
The gap is almost entirely explained by `bufio.Reader` buffer sizing: the iterator uses `bufio.NewReader`'s 4096-byte default; the channel version uses 256 bytes. Equalize the buffer size and the heap difference largely disappears. This is an implementation detail, not an architectural advantage.

#### Decision

The **iterator lexer is the production path** (`ReadAddress`, `ParseAddress`).

The Pike-style channel lexer remains in the codebase (`ReadAddressOptimized`, `chan_lexer.go`) as a documented alternative and benchmark baseline, but it is not recommended for production use on short inputs.

#### When the channel approach would be the right choice

- Long, streaming inputs where lexing and parsing can meaningfully overlap on different cores
- Complex state machines where the `func(*lexer) stateFn` pattern is materially clearer than a `switch` in `Next()`
- Workloads where natural backpressure via a buffered channel is desirable

None of these apply to single email address parsing.

---
