.PHONY: build test lint fmt vet coverage clean

BINARY := kumokagi
CMD := ./cmd/kumokagi

build:
	go build -o bin/$(BINARY) $(CMD)

test:
	go test -race ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | grep total

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

clean:
	rm -rf bin/ coverage.out

test-integration:
	go test -race -tags=integration ./...
