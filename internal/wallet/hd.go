package wallet

import (
	"fmt"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

const hardenedOffset uint32 = 0x80000000

// Фиксированная часть пути деривации ethereum: purpose=44', coin_type=60' (SLIP-44).
const (
	purpose  uint32 = 44
	coinType uint32 = 60
)

func masterKeyFromMnemonic(mnemonic string) (*bip32.Key, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	// пустой passphrase - стандарт для таких кошельков
	seed := bip39.NewSeed(mnemonic, "")

	return bip32.NewMasterKey(seed)
}

// derivePrivateKey проходит m/44'/60'/account'/change/address_index и возвращает
// 32-байтный приватный ключ в листе.
func derivePrivateKey(master *bip32.Key, p Path) ([]byte, error) {
	indices := []uint32{
		purpose + hardenedOffset,
		coinType + hardenedOffset,
		p.Account + hardenedOffset,
		p.Change,
		p.AddressIndex,
	}

	key := master
	for _, idx := range indices {
		child, err := key.NewChildKey(idx)
		if err != nil {
			return nil, fmt.Errorf("derive child %d: %w", idx, err)
		}
		key = child
	}

	// bip32 хранит сырой 32-байтный приватный ключ в Key; страхуемся от ведущего
	// нулевого 0x00 байта-паддинга на всякий случай.
	keyBytes := key.Key
	if len(keyBytes) == 33 && keyBytes[0] == 0x00 {
		keyBytes = keyBytes[1:]
	}

	return keyBytes, nil
}
