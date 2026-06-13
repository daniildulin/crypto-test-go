# Wallet Service (Go)

Оффлайн wallet-сервис из тестового задания «Blockchain Integrator». Хранит сид-фразы,
деривит HD-адреса, валидирует адреса и **подписывает транзакции оффлайн**. В RPC-ноду
не ходит — broadcast делает Laravel-сторона.

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

## Архитектура

```
cmd/walletd        точка входа: загрузка конфига, сборка signer'а, запуск, graceful shutdown
internal/config    загрузка config.json + валидация
internal/wallet    интерфейс Signer, HD-деривация, EthSigner (go-ethereum)
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

Текущая реализация (`EthSigner`) — **чистый Go** (`go-ethereum` + `tyler-smith/go-bip39`
+ `go-bip32`), поэтому собирается с `CGO_ENABLED=0` и не требует нативных библиотек.
Реализацию поверх trustwallet/wallet-core можно подставить за тем же интерфейсом, не трогая
слой API.

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

```bash
cp config.example.json config.json   # потом впишите свой реальный mnemonic
make run                             # или: go run ./cmd/walletd -config config.json
```

Docker:

```bash
make docker
docker run --rm -p 8000:8000 -v "$PWD/config.json:/config.json:ro" crypto-test-walletd
```

## Тесты

```bash
make test   # CGO_ENABLED=0 go test ./...
```

Покрытие: HD-деривация против известных hardhat-векторов, EIP-55 валидация, EIP-1559
подпись (декодируем + восстанавливаем sender обратно в derived-адрес, проверка
детерминизма), валидация конфига и HTTP-handlers.

## Заметки по безопасности

- Mnemonic живут только в `config.json` / памяти; подпись полностью оффлайн.
- Сервис не отдаёт ключевой материал наружу — только адреса и подписанные транзакции.
- Биндимся на `127.0.0.1` (или внутреннюю сеть), наружу сервис торчать не должен.
