# Боевой образ: signer поверх нативной trustwallet/wallet-core (cgo).
# Запасная сборка без cgo - в Dockerfile.native.

# ---------------------------------------------------------------------------
# Стадия 1: сборка нативной wallet-core, версия запинена на 2.6.34.
# Базой берём официальный dev-образ wallet-core: в нём уже стоит нужный
# тулчейн (clang-10 + libc++), boost и собранный protobuf 3.14 - ровно те
# версии, что требует 2.6.34. Переключаем вендоренный исходник на тег с
# поддержкой EIP-1559 и пересобираем только саму библиотеку (без тестов).
#
# Почему 2.6.34: это последняя ветка wallet-core на чистом C++ (до перехода
# на Rust-воркспейс в 3.x), в которой уже есть EIP-1559. Сборка легче и
# воспроизводимее, в образ не тащится Rust-тулчейн.
# ---------------------------------------------------------------------------
# buildwc - свежий каталог сборки (существующий build/ в образе сконфигурён под
# Make и старый исходник; не реюзаем, чтобы не ловить конфликт генераторов).
# Зависимости (protobuf 3.14, boost) берутся из готового build/local.
FROM trustwallet/wallet-core AS wcbuild
RUN cd /wallet-core \
 && git fetch --tags --force --depth 1 origin refs/tags/2.6.34:refs/tags/2.6.34 \
 && git checkout 2.6.34 \
 && tools/generate-files \
 && cmake -H. -Bbuildwc -GNinja -DCMAKE_BUILD_TYPE=Release \
 && ninja -Cbuildwc TrustWalletCore TrezorCrypto

# ---------------------------------------------------------------------------
# Стадия 2: сборка нашего сервиса с cgo поверх wallet-core.
# Тот же образ - значит тот же clang-10/libc++ ABI, что и у библиотеки;
# добавляем только Go. Здесь же прогоняем тесты wallet-core (-tags walletcore).
# ---------------------------------------------------------------------------
FROM wcbuild AS gobuild
ARG GO_VERSION=1.24.4
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz
ENV PATH=/usr/local/go/bin:$PATH \
    CGO_ENABLED=1 \
    CC=clang-10 \
    CXX=clang++-10 \
    CGO_CFLAGS="-I/wallet-core/include" \
    CGO_LDFLAGS="-L/wallet-core/buildwc -L/wallet-core/buildwc/trezor-crypto -L/wallet-core/build/local/lib -lTrustWalletCore -lprotobuf -lTrezorCrypto -lc++ -lc++abi -lpthread -lm"
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test -tags walletcore ./... \
 && go build -tags walletcore -trimpath -o /walletd ./cmd/walletd

# ---------------------------------------------------------------------------
# Стадия 3: рантайм. cgo => не distroless-static; берём distroless/cc (glibc)
# и докладываем динамические libc++ из стадии сборки.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/cc-debian12
COPY --from=wcbuild /usr/lib/x86_64-linux-gnu/libc++.so.1 /usr/lib/x86_64-linux-gnu/libc++abi.so.1 /usr/lib/x86_64-linux-gnu/
COPY --from=gobuild /walletd /walletd
EXPOSE 8000

# config.json с mnemonic монтируется в рантайме, не вшит в образ.
# signer по умолчанию - walletcore (этот образ для того и собран).
ENTRYPOINT ["/walletd", "-config", "/config.json"]
