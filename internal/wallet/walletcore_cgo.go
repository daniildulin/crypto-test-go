//go:build walletcore

package wallet

/*
#include <stdlib.h>
#include <TrustWalletCore/TWHDWallet.h>
#include <TrustWalletCore/TWPrivateKey.h>
#include <TrustWalletCore/TWPublicKey.h>
#include <TrustWalletCore/TWAnyAddress.h>
#include <TrustWalletCore/TWAnySigner.h>
#include <TrustWalletCore/TWString.h>
#include <TrustWalletCore/TWData.h>
#include <TrustWalletCore/TWHash.h>
#include <TrustWalletCore/TWCoinType.h>
*/
import "C"

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"unsafe"

	"github.com/daniildulin/crypto-test/internal/wallet/wcproto/ethereum"
	"google.golang.org/protobuf/proto"
)

// coinEthereum - TWCoinType для Ethereum (SLIP-44 coin type 60).
const coinEthereum = C.TWCoinTypeEthereum

// walletCoreSigner - боевой Signer поверх нативной trustwallet/wallet-core.
// Деривация ключей, формат адреса и подпись (включая EIP-1559) делаются
// библиотекой, а не нашим кодом.
type walletCoreSigner struct {
	// gate -> TWHDWallet. Кошелёк (а значит и seed) создаётся один раз на старте
	// и живёт всё время работы сервиса - деривация seed'а дорогая.
	wallets map[string]*C.struct_TWHDWallet
}

// newWalletCoreSigner строит HD-кошелёк wallet-core для каждого gate.
// Падает сразу на плохом mnemonic (CreateWithMnemonic возвращает NULL).
func newWalletCoreSigner(mnemonics map[string]string) (Signer, error) {
	empty := twStringCreate("")
	defer C.TWStringDelete(empty)

	wallets := make(map[string]*C.struct_TWHDWallet, len(mnemonics))
	for name, mnemonic := range mnemonics {
		ms := twStringCreate(mnemonic)
		w := C.TWHDWalletCreateWithMnemonic(ms, empty)
		C.TWStringDelete(ms)
		if w == nil {
			return nil, fmt.Errorf("gate %q: invalid mnemonic", name)
		}
		wallets[name] = w
	}

	return &walletCoreSigner{wallets: wallets}, nil
}

func (s *walletCoreSigner) DeriveAddress(gate string, p Path) (string, error) {
	w, ok := s.wallets[gate]
	if !ok {
		return "", fmt.Errorf("unknown gate %q", gate)
	}

	key := C.TWHDWalletGetDerivedKey(w, coinEthereum,
		C.uint32_t(p.Account), C.uint32_t(p.Change), C.uint32_t(p.AddressIndex))
	defer C.TWPrivateKeyDelete(key)

	// uncompressed pubkey - дальше wallet-core сам сделает keccak + EIP-55 checksum
	pub := C.TWPrivateKeyGetPublicKeySecp256k1(key, C.bool(false))
	defer C.TWPublicKeyDelete(pub)

	addr := C.TWAnyAddressCreateWithPublicKey(pub, coinEthereum)
	defer C.TWAnyAddressDelete(addr)

	desc := C.TWAnyAddressDescription(addr)
	defer C.TWStringDelete(desc)

	return twStringGo(desc), nil
}

func (s *walletCoreSigner) ValidateAddress(gate, address string) (bool, error) {
	if _, ok := s.wallets[gate]; !ok {
		return false, fmt.Errorf("unknown gate %q", gate)
	}

	str := twStringCreate(address)
	defer C.TWStringDelete(str)

	return bool(C.TWAnyAddressIsValid(str, coinEthereum)), nil
}

func (s *walletCoreSigner) SignTx(gate string, p Path, tp TxParams) (SignedTx, error) {
	w, ok := s.wallets[gate]
	if !ok {
		return SignedTx{}, fmt.Errorf("unknown gate %q", gate)
	}

	// Валидируем вход теми же хелперами, что и native-реализация, чтобы API вёл
	// себя одинаково независимо от выбранного signer'а.
	value, err := parseWei(tp.ValueWei, "value_wei")
	if err != nil {
		return SignedTx{}, err
	}
	tip, err := parseWei(tp.MaxPriorityFeePerGasWei, "max_priority_fee_per_gas_wei")
	if err != nil {
		return SignedTx{}, err
	}
	feeCap, err := parseWei(tp.MaxFeePerGasWei, "max_fee_per_gas_wei")
	if err != nil {
		return SignedTx{}, err
	}
	data, err := decodeData(tp.Data)
	if err != nil {
		return SignedTx{}, err
	}
	if tp.ChainID == 0 {
		return SignedTx{}, fmt.Errorf("chain_id is required")
	}
	if !isValidEthAddress(tp.To) {
		return SignedTx{}, fmt.Errorf("invalid to address %q", tp.To)
	}

	key := C.TWHDWalletGetDerivedKey(w, coinEthereum,
		C.uint32_t(p.Account), C.uint32_t(p.Change), C.uint32_t(p.AddressIndex))
	defer C.TWPrivateKeyDelete(key)

	privData := C.TWPrivateKeyData(key)
	defer C.TWDataDelete(privData)

	// EIP-1559 (Enveloped, тип 0x02). ContractGeneric описывает и нативный
	// перевод (data пустая), и вызов контракта (data = calldata от Laravel) -
	// одинаково. Денежные поля - big-endian байты 256-битных чисел.
	input := &ethereum.SigningInput{
		ChainId:               new(big.Int).SetUint64(tp.ChainID).Bytes(),
		Nonce:                 new(big.Int).SetUint64(tp.Nonce).Bytes(),
		TxMode:                ethereum.TransactionMode_Enveloped,
		GasLimit:              new(big.Int).SetUint64(tp.GasLimit).Bytes(),
		MaxInclusionFeePerGas: tip.Bytes(),
		MaxFeePerGas:          feeCap.Bytes(),
		ToAddress:             tp.To,
		PrivateKey:            twDataGo(privData),
		Transaction: &ethereum.Transaction{
			TransactionOneof: &ethereum.Transaction_ContractGeneric_{
				ContractGeneric: &ethereum.Transaction_ContractGeneric{
					Amount: value.Bytes(),
					Data:   data,
				},
			},
		},
	}

	raw, err := proto.Marshal(input)
	if err != nil {
		return SignedTx{}, fmt.Errorf("marshal signing input: %w", err)
	}

	inputData := twDataCreate(raw)
	defer C.TWDataDelete(inputData)

	outData := C.TWAnySignerSign(inputData, coinEthereum)
	defer C.TWDataDelete(outData)

	var out ethereum.SigningOutput
	if err := proto.Unmarshal(twDataGo(outData), &out); err != nil {
		return SignedTx{}, fmt.Errorf("unmarshal signing output: %w", err)
	}
	if len(out.GetEncoded()) == 0 {
		return SignedTx{}, fmt.Errorf("wallet-core returned empty encoded tx")
	}

	// tx_hash = keccak256(подписанная RLP), считаем тем же wallet-core.
	encData := twDataCreate(out.GetEncoded())
	defer C.TWDataDelete(encData)
	hashData := C.TWHashKeccak256(encData)
	defer C.TWDataDelete(hashData)

	return SignedTx{
		TxHash:   "0x" + hex.EncodeToString(twDataGo(hashData)),
		SignedTx: "0x" + hex.EncodeToString(out.GetEncoded()),
	}, nil
}

// --- мостики Go <-> C для опаковых TWString / TWData (как в samples/go) ---

// twStringCreate: Go string -> C.TWString (нужно освобождать TWStringDelete).
func twStringCreate(s string) unsafe.Pointer {
	c := C.CString(s)
	defer C.free(unsafe.Pointer(c))
	return unsafe.Pointer(C.TWStringCreateWithUTF8Bytes(c))
}

// twStringGo: C.TWString -> Go string.
func twStringGo(s unsafe.Pointer) string {
	return C.GoString(C.TWStringUTF8Bytes(s))
}

// twDataCreate: Go []byte -> C.TWData (нужно освобождать TWDataDelete).
func twDataCreate(b []byte) unsafe.Pointer {
	if len(b) == 0 {
		return unsafe.Pointer(C.TWDataCreateWithBytes((*C.uchar)(nil), C.size_t(0)))
	}
	c := C.CBytes(b)
	defer C.free(c)
	return unsafe.Pointer(C.TWDataCreateWithBytes((*C.uchar)(c), C.size_t(len(b))))
}

// twDataGo: C.TWData -> Go []byte.
func twDataGo(d unsafe.Pointer) []byte {
	return C.GoBytes(unsafe.Pointer(C.TWDataBytes(d)), C.int(C.TWDataSize(d)))
}
