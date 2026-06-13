# стадия сборки
FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/walletd ./cmd/walletd

# стадия запуска - статический бинарь на distroless
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/walletd /walletd
EXPOSE 8000

# config.json с mnemonic монтируется в рантайме, не вшит в образ
ENTRYPOINT ["/walletd", "-config", "/config.json"]
