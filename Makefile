
.PHONY: pre-checks fmt vet test
pre-checks: fmt vet test

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test -v ./...

.PHONY: bench-addr-parser
bench-addr-parser:
	go test ./email/address/... -bench=. -benchmem -count=3