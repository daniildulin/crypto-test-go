.PHONY: build test vet run tidy docker

# без cgo: чистый Go secp256k1, статический бинарь
export CGO_ENABLED := 0

build:
	go build -trimpath -o bin/walletd ./cmd/walletd

test:
	go test ./...

vet:
	go vet ./...

run: build
	./bin/walletd -config config.json

tidy:
	go mod tidy

docker:
	docker build -t crypto-test-walletd .
