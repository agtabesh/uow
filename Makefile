GOBIN = $(shell go env GOPATH)/bin

.PHONY: test lint coverage build tidy clean

test:
	go test ./... -v

lint:
	$(GOBIN)/golangci-lint run

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

build:
	go build ./...

tidy:
	go mod tidy

clean:
	rm -f coverage.out coverage.html
