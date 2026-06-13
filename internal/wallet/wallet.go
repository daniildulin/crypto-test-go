package wallet

// Path - хвост BIP44-пути m/44'/60'/account'/change/address_index.
type Path struct {
	Account      uint32
	Change       uint32
	AddressIndex uint32
}

// TxParams описывает EIP-1559 транзакцию для оффлайн-подписи.
// Все денежные поля - десятичные строки в wei (без float).
type TxParams struct {
	To                      string
	ValueWei                string
	Data                    string
	Nonce                   uint64
	ChainID                 uint64
	GasLimit                uint64
	MaxFeePerGasWei         string
	MaxPriorityFeePerGasWei string
}

// SignedTx - результат оффлайн-подписи.
type SignedTx struct {
	TxHash   string
	SignedTx string
}

// Signer абстрагирует управление ключами и подпись. Реализацию на go-ethereum
// (EthSigner) можно заменить на реализацию поверх trustwallet/wallet-core без
// изменений в слое API.
type Signer interface {
	DeriveAddress(gate string, p Path) (string, error)
	ValidateAddress(gate, address string) (bool, error)
	SignTx(gate string, p Path, tx TxParams) (SignedTx, error)
}
