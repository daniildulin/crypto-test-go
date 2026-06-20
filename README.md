# Wallet Service (Go)

Оффлайн wallet-сервис из тестового задания «Blockchain Integrator». Хранит сид-фразы,
деривит HD-адреса, валидирует адреса и **подписывает транзакции оффлайн**. В RPC-ноду
не ходит — broadcast делает Laravel-сторона.

Криптография (деривация, адрес, подпись) идёт через **trustwallet/wallet-core** —
как и требует ТЗ. Есть запасная чистая Go-реализация (см. «Реализации signer'а»).

## Эндпоинты

Все эндпоинты — `POST` с JSON-телом.

### `POST /api/v1/createaddress`
```json
{ "gate": "ethereum", "account": 0, "change": 0, "address_index": 15 }
```
→ `{ "address": "0x..." }`

### `POST /api/v1/validateaddress`
```json
{ "gate": "ethereum", "address": "0x..." }
```
→ `{ "valid": true }`

Проверка формата + EIP-55 checksum для mixed-case ввода.

### `POST /api/v1/tx` (оффлайн-подпись)
```json
{
  "gate": "ethereum",
  "account": 0, "change": 0, "address_index": 15,
  "tx_params": {
    "to": "0x...",
    "value_wei": "0",
    "data": "0x...",
    "nonce": 13,
    "chain_id": 11155111,
    "gas_limit": 90000,
    "max_fee_per_gas_wei": "30000000000",
    "max_priority_fee_per_gas_wei": "1500000000"
  }
}
```
→ `{ "tx_hash": "0x...", "signed_tx": "0x02..." }`

Собирает EIP-1559 (`DynamicFeeTx`, тип `0x02`) транзакцию. Все суммы — десятичные строки
в wei, без float.

## HD-деривация

`m / 44' / 60' / account' / change / address_index` (SLIP-44 coin type 60 = Ethereum).
Фиксированный префикс `44'/60'` живёт в сервисе; API передаёт только хвост пути.
В wallet-core этому соответствует `TWHDWalletGetDerivedKey(coin, account, change, index)`.

## Архитектура

```
cmd/walletd        точка входа: загрузка конфига, выбор signer'а, запуск, graceful shutdown
internal/config    загрузка config.json + валидация
internal/wallet    интерфейс Signer + две реализации (wallet-core / native) + HD-логика
internal/wallet/wcproto/ethereum  сгенерированный Go-protobuf под wallet-core (Ethereum.proto)
internal/api       HTTP-слой: dto, handlers, роутер (stdlib mux, method routing)
```

Логика ключей и подписи спрятана за интерфейсом `wallet.Signer`:

```go
type Signer interface {
    DeriveAddress(gate string, p Path) (string, error)
    ValidateAddress(gate, address string) (bool, error)
    SignTx(gate string, p Path, tx TxParams) (SignedTx, error)
}
```

## Реализации signer'а

Реализация выбирается флагом `--signer` (по умолчанию `walletcore`):

- **`walletcore`** — боевая. Деривация, адрес и подпись (включая EIP-1559) делаются
  нативной библиотекой `trustwallet/wallet-core` через cgo. Транзакция собирается как
  `Ethereum.Proto.SigningInput` (`tx_mode = Enveloped`, `ContractGeneric{amount,data}` —
  единообразно покрывает и нативный ETH-перевод, и ERC20 calldata от Laravel),
  подписывается `TWAnySignerSign`. Компилируется **только** со сборочным тегом
  `walletcore` (нужна нативная библиотека) — см. «Сборка wallet-core».

- **`native`** — запасная. Чистый Go на `go-ethereum` + `go-bip39`/`go-bip32`, без cgo,
  собирается с `CGO_ENABLED=0` в статический бинарь. Удобна для локального запуска и CI,
  где нет нативной библиотеки.

Выбор разведён по build-тегу: обычная сборка (`go build ./...`) не тянет cgo и содержит
только `native`; боевой образ собирается с `-tags walletcore` и содержит обе реализации —
флаг `--signer` переключает их в рантайме.

## Сборка wallet-core

`trustwallet/wallet-core` — это нативная C++ библиотека, она подключается из Go через cgo;
чистого Go-варианта нет. Готового бинарного образа с EIP-1559 не существует (единственный
публичный образ `trustwallet/wallet-core:latest` — 2021 года, поддерживает только legacy-
транзакции), поэтому библиотека **собирается из исходников** в Docker, версия запинена.

**Версия 2.6.34.** Это последняя ветка wallet-core на чистом C++ (в 3.x появился
Rust-воркспейс), в которой **уже есть EIP-1559**. Так сборка легче и воспроизводимее, а в
образ не тащится Rust-тулчейн. Boost / protobuf / clang берём готовыми из официального dev-
образа wallet-core, переключаем исходник на тег 2.6.34 и пересобираем только саму
библиотеку.

Сборка с cgo использует `clang` + `libstdc++` (тот же тулчейн, что и у wallet-core/protobuf),
поэтому рантайм-образ — не distroless-static, а `distroless/cc` (в нём есть glibc + libstdc++).
Запасная `native`-сборка остаётся полностью статической (см. `Dockerfile.native`).

## Конфиг

`config.json` (см. `config.example.json`):

```json
{
  "config": { "host": "127.0.0.1", "port": 8000 },
  "gates": [ { "name": "ethereum", "mnemonic": "<seed phrase>" } ]
}
```

`config.json` содержит боевые mnemonic и **в git не коммитится**. В примере — публичный
тестовый mnemonic hardhat/anvil, не отправляйте на эти адреса реальные средства.

## Запуск

Локально (native signer, без cgo и без нативной библиотеки):

```bash
cp config.example.json config.json   # потом впишите свой реальный mnemonic
make run                             # go run ... -signer native
```

Боевой образ (signer на wallet-core, библиотека собирается из исходников):

```bash
make docker
docker run --rm -p 8000:8000 -v "$PWD/config.json:/config.json:ro" crypto-test-walletd
```

Запасной образ без cgo (native signer, статический бинарь):

```bash
make docker-native
```

## Тесты

```bash
make test            # CGO_ENABLED=0 go test ./...  - native-путь, быстро, без библиотеки
make test-walletcore # тесты wallet-core (-tags walletcore) внутри Docker-сборки
```

Покрытие native: HD-деривация против известных hardhat-векторов, EIP-55 валидация,
EIP-1559 подпись (декодируем + восстанавливаем sender обратно в derived-адрес, проверка
детерминизма), валидация конфига и HTTP-handlers. Тесты wallet-core дополнительно
кросс-проверяют подпись: подписываем через wallet-core, декодируем и восстанавливаем
sender через go-ethereum, сверяем с derived-адресом — две независимые реализации должны
сходиться.

## Заметки по безопасности

- Mnemonic живут только в `config.json` / памяти; подпись полностью оффлайн.
- Сервис не отдаёт ключевой материал наружу — только адреса и подписанные транзакции.
- Биндимся на `127.0.0.1` (или внутреннюю сеть), наружу сервис торчать не должен.
