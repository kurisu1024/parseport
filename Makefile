
.PHONY: pre-checks fmt vet test
pre-checks: fmt vet test

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test -v ./...

