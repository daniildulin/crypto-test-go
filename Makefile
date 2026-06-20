.PHONY: build test vet run tidy docker docker-native test-walletcore

# Локальные цели - без cgo: чистый Go (native signer), статический бинарь.
# Боевой signer (wallet-core) требует нативной библиотеки и собирается в Docker.
export CGO_ENABLED := 0

build:
	go build -trimpath -o bin/walletd ./cmd/walletd

test:
	go test ./...

vet:
	go vet ./...

# Локальный запуск без wallet-core: native signer (cgo не нужен).
run: build
	./bin/walletd -config config.json -signer native

tidy:
	go mod tidy

# Боевой образ: собирает wallet-core 2.6.34 из исходников и линкует cgo.
docker:
	docker build -t crypto-test-walletd .

# Запасной образ: чистый Go без cgo/wallet-core.
docker-native:
	docker build -f Dockerfile.native -t crypto-test-walletd-native .

# Тесты wallet-core (-tags walletcore) гоняются на стадии gobuild внутри Docker,
# поэтому успешный build == зелёные тесты wallet-core.
test-walletcore: docker
